package conformance

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

func run(cmd string, args ...string) (string, error) {
	c := exec.Command(cmd, args...)
	b, err := c.CombinedOutput()
	return string(b), err
}

func runStdin(cmd string, args []string, stdin string) (string, error) {
	c := exec.Command(cmd, args...)
	c.Stdin = strings.NewReader(stdin)
	b, err := c.CombinedOutput()
	return string(b), err
}

func kubectl(args ...string) (string, error) { return run("kubectl", args...) }

func kubectlApply(yaml string) (string, error) {
	return runStdin("kubectl", []string{"apply", "-f", "-"}, yaml)
}

func kubectlCreate(yaml string) (string, error) {
	return runStdin("kubectl", []string{"create", "-f", "-"}, yaml)
}

func kubectlDelete(yaml string) (string, error) {
	return runStdin("kubectl", []string{"delete", "--ignore-not-found", "-f", "-"}, yaml)
}

func clientcmdPath() string {
	if p := os.Getenv("KUBECONFIG"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return home + "/.kube/config"
}

func newClient() client.Client {
	cfg, err := clientcmd.BuildConfigFromFlags("", clientcmdPath())
	Expect(err).ToNot(HaveOccurred())
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(hermesv1.AddToScheme(scheme))
	c, err := client.New(cfg, client.Options{Scheme: scheme})
	Expect(err).ToNot(HaveOccurred())
	return c
}

func newKubeClient() *kubernetes.Clientset {
	cfg, err := clientcmd.BuildConfigFromFlags("", clientcmdPath())
	Expect(err).ToNot(HaveOccurred())
	cs, err := kubernetes.NewForConfig(cfg)
	Expect(err).ToNot(HaveOccurred())
	return cs
}

func waitForInstanceReady(ctx context.Context, c client.Client, ns, name string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		inst := &hermesv1.HermesInstance{}
		err := c.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, inst)
		if err == nil && hasReadyTrue(inst) {
			return
		}
		time.Sleep(2 * time.Second)
	}
	Fail(fmt.Sprintf("HermesInstance %s/%s did not become Ready within %s", ns, name, timeout))
}

func hasReadyTrue(inst *hermesv1.HermesInstance) bool {
	for _, cond := range inst.Status.Conditions {
		if cond.Type == "Ready" && cond.Status == "True" {
			return true
		}
	}
	return false
}

func forceRequeue(ctx context.Context, c client.Client, ns, name string) {
	inst := &hermesv1.HermesInstance{}
	Expect(c.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, inst)).To(Succeed())
	if inst.Annotations == nil {
		inst.Annotations = map[string]string{}
	}
	inst.Annotations["hermes.agent/conformance-poke"] = fmt.Sprintf("%d", time.Now().UnixNano())
	Expect(c.Update(ctx, inst)).To(Succeed())
}

type metaTuple struct {
	Generation      int64
	ResourceVersion string
}

type resourceFingerprint struct {
	StatefulSet        metaTuple
	Service            metaTuple
	WorkspaceConfigMap metaTuple
	GatewayConfigMap   metaTuple
	PVC                metaTuple
}

func captureFingerprint(ctx context.Context, c client.Client, ns, name string) resourceFingerprint {
	fp := resourceFingerprint{}
	sts := &appsv1.StatefulSet{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, sts); err == nil {
		fp.StatefulSet = metaTuple{sts.Generation, sts.ResourceVersion}
	}
	svc := &corev1.Service{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, svc); err == nil {
		fp.Service = metaTuple{svc.Generation, svc.ResourceVersion}
	}
	cm := &corev1.ConfigMap{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: ns, Name: name + "-workspace"}, cm); err == nil {
		fp.WorkspaceConfigMap = metaTuple{cm.Generation, cm.ResourceVersion}
	}
	if err := c.Get(ctx, types.NamespacedName{Namespace: ns, Name: name + "-config"}, cm); err == nil {
		fp.GatewayConfigMap = metaTuple{cm.Generation, cm.ResourceVersion}
	}
	pvc := &corev1.PersistentVolumeClaim{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: ns, Name: name + "-data"}, pvc); err == nil {
		fp.PVC = metaTuple{pvc.Generation, pvc.ResourceVersion}
	}
	return fp
}

func expectFingerprintUnchanged(before, after resourceFingerprint) {
	check := func(fieldName string, b, a metaTuple) {
		Expect(a.Generation).To(Equal(b.Generation),
			fmt.Sprintf("%s.metadata.generation changed: %d -> %d (idempotency broken)", fieldName, b.Generation, a.Generation))
		Expect(a.ResourceVersion).To(Equal(b.ResourceVersion),
			fmt.Sprintf("%s.metadata.resourceVersion changed: %s -> %s (idempotency broken)", fieldName, b.ResourceVersion, a.ResourceVersion))
	}
	check("StatefulSet", before.StatefulSet, after.StatefulSet)
	check("Service", before.Service, after.Service)
	check("WorkspaceConfigMap", before.WorkspaceConfigMap, after.WorkspaceConfigMap)
	check("GatewayConfigMap", before.GatewayConfigMap, after.GatewayConfigMap)
	check("PVC", before.PVC, after.PVC)
}

func readFile(path string) string {
	b, err := os.ReadFile(path)
	Expect(err).ToNot(HaveOccurred(), "reading %s", path)
	return string(b)
}

// IsNotFoundError returns true if err is a Kubernetes not-found error.
func IsNotFoundError(err error) bool { return apierrors.IsNotFound(err) }

func freshNamespace(prefix string) string {
	ns := fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	out, err := kubectl("create", "namespace", ns)
	Expect(err).ToNot(HaveOccurred(), "create ns: %s", out)
	return ns
}

func deleteNamespace(ns string) {
	_, _ = kubectl("delete", "namespace", ns, "--ignore-not-found", "--wait=false")
}

// Quiet "imported and not used" linter for vendored types referenced in
// (yet-unwritten) sibling test files. Safe to remove later.
var _ = newKubeClient
var _ = kubectlApply
var _ = kubectlDelete
var _ = IsNotFoundError
var _ = kubectlCreate
var _ = readFile
var _ = waitForInstanceReady
var _ = forceRequeue
var _ = freshNamespace
var _ = deleteNamespace

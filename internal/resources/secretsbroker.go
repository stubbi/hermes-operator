package resources

import (
	"encoding/json"
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"

	hermesv1 "github.com/paperclipinc/hermes-operator/api/v1"
	"github.com/paperclipinc/hermes-operator/internal/secretsbroker"
)

// OperatorImage is the running operator's own image, set at startup from the
// HERMES_OPERATOR_IMAGE env var. The secrets broker init container copies the
// resolver (the operator binary itself) out of it.
var OperatorImage string

const (
	// SecretsBrokerVolume holds the resolver binary for the agent container.
	SecretsBrokerVolume = "secrets-broker-bin"
	// SecretsBrokerInitName is the init container that installs the resolver.
	SecretsBrokerInitName = "init-secrets-broker"
	secretsBrokerDir      = "/opt/hermes-secrets-broker"
	secretsBrokerBinary   = secretsBrokerDir + "/resolver"
	// secretsBrokerHeaderEnvPrefix names the env vars that carry valueFrom
	// header values; the rendered config references them as ${...}.
	secretsBrokerHeaderEnvPrefix = "HERMES_SECRETS_BROKER_HEADER_"
	// operatorUID is the distroless nonroot user of the operator image.
	operatorUID = int64(65532)
)

// DefaultSecretsBrokerEntrypoint is the upstream hermes-agent image ENTRYPOINT.
var DefaultSecretsBrokerEntrypoint = []string{"/init", "/opt/hermes/docker/main-wrapper.sh"}

// SecretsBrokerConfigJSON renders spec.secretsBroker into the document the
// resolver reads from HERMES_SECRETS_BROKER_CONFIG. Headers sourced from a
// Secret become ${HERMES_SECRETS_BROKER_HEADER_<n>} placeholders backed by a
// secretKeyRef env var, so no credential is ever rendered into the pod spec.
func SecretsBrokerConfigJSON(sb *hermesv1.SecretsBrokerSpec) (string, []corev1.EnvVar, error) {
	cfg := secretsbroker.Config{
		URL:               sb.URL,
		Method:            sb.Method,
		EnvPrefix:         sb.EnvPrefix,
		AllowInsecureHTTP: sb.AllowInsecureHTTP,
	}
	if sb.TimeoutSeconds != nil {
		cfg.TimeoutSeconds = int(*sb.TimeoutSeconds)
	}
	if sb.MaxAttempts != nil {
		cfg.MaxAttempts = int(*sb.MaxAttempts)
	}
	var env []corev1.EnvVar
	if len(sb.Headers) > 0 {
		cfg.Headers = make(map[string]string, len(sb.Headers))
	}
	for i, h := range sb.Headers {
		if h.ValueFrom != nil {
			name := secretsBrokerHeaderEnvPrefix + strconv.Itoa(i)
			ref := h.ValueFrom.SecretKeyRef
			env = append(env, corev1.EnvVar{
				Name:      name,
				ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &ref},
			})
			cfg.Headers[h.Name] = "${" + name + "}"
			continue
		}
		cfg.Headers[h.Name] = h.Value
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return "", nil, err
	}
	return string(raw), env, nil
}

// applySecretsBroker wires spec.secretsBroker into the pod: an init container
// copies the resolver into a shared emptyDir, and the agent container's
// entrypoint becomes `resolver secrets-broker exec -- <entrypoint>` so the
// resolver fetches the secrets and execs the real entrypoint in place (s6
// /init stays PID 1). Args are left untouched.
func applySecretsBroker(inst *hermesv1.HermesInstance, c *corev1.Container) (*corev1.Container, *corev1.Volume) {
	sb := inst.Spec.SecretsBroker
	if sb == nil {
		return nil, nil
	}

	entry := sb.Entrypoint
	if len(entry) == 0 {
		entry = DefaultSecretsBrokerEntrypoint
	}
	c.Command = append([]string{secretsBrokerBinary, secretsbroker.Subcommand, "exec", "--"}, entry...)
	c.VolumeMounts = append(c.VolumeMounts, corev1.VolumeMount{
		Name:      SecretsBrokerVolume,
		MountPath: secretsBrokerDir,
		ReadOnly:  true,
	})

	init := &corev1.Container{
		Name:                     SecretsBrokerInitName,
		ImagePullPolicy:          corev1.PullIfNotPresent,
		TerminationMessagePath:   "/dev/termination-log",
		TerminationMessagePolicy: corev1.TerminationMessageReadFile,
		VolumeMounts: []corev1.VolumeMount{{
			Name:      SecretsBrokerVolume,
			MountPath: secretsBrokerDir,
		}},
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: Ptr(false),
			ReadOnlyRootFilesystem:   Ptr(true),
			RunAsNonRoot:             Ptr(true),
			RunAsUser:                Ptr(operatorUID),
			Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
		},
	}

	cfgJSON, headerEnv, err := SecretsBrokerConfigJSON(sb)
	image := sb.ResolverImage
	if image == "" {
		image = OperatorImage
	}
	switch {
	case err != nil:
		failInit(init, imageRef(inst), fmt.Sprintf("spec.secretsBroker could not be rendered: %v", err))
	case image == "":
		// Fail closed and loudly rather than starting the agent without its
		// secrets: the operator does not know which image to copy from.
		failInit(init, imageRef(inst), "spec.secretsBroker needs a resolver image: set spec.secretsBroker.resolverImage, or run the operator with HERMES_OPERATOR_IMAGE (the Helm chart and OLM bundle set it)")
	default:
		init.Image = image
		init.Command = []string{"/manager", secretsbroker.Subcommand, "install", secretsBrokerBinary}
	}

	c.Env = append(c.Env, headerEnv...)
	c.Env = append(c.Env, corev1.EnvVar{Name: secretsbroker.ConfigEnv, Value: cfgJSON})

	vol := &corev1.Volume{
		Name:         SecretsBrokerVolume,
		VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
	}
	return init, vol
}

func failInit(init *corev1.Container, image, msg string) {
	init.Image = image
	init.Command = []string{"/bin/sh", "-c", `echo "$0" >&2; echo "$0" > /dev/termination-log; exit 1`, msg}
	// The agent image runs its init as root by default; the message write
	// needs no privileges, so keep the hardened context but drop the uid pin.
	init.SecurityContext.RunAsUser = nil
	init.SecurityContext.RunAsNonRoot = nil
}

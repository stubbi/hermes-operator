package controller

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"

	"github.com/paperclipinc/hermes-operator/internal/resources"
)

func seedPod(st corev1.ContainerStatus) *corev1.Pod {
	st.Name = resources.ConfigSeedContainerName
	return &corev1.Pod{Status: corev1.PodStatus{InitContainerStatuses: []corev1.ContainerStatus{
		{Name: "init-restore", State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0}}},
		st,
	}}}
}

func TestConfigSeedFailure(t *testing.T) {
	t.Parallel()
	failed := &corev1.ContainerStateTerminated{ExitCode: 1, Message: "[config-migrate] ERROR: [Errno 30] Read-only file system\n"}

	cases := []struct {
		name    string
		pod     *corev1.Pod
		want    bool
		wantMsg string
	}{
		{name: "no statuses yet", pod: &corev1.Pod{}, want: false},
		{name: "still running", pod: seedPod(corev1.ContainerStatus{State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}), want: false},
		{name: "completed", pod: seedPod(corev1.ContainerStatus{State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0}}}), want: false},
		{name: "failed now", pod: seedPod(corev1.ContainerStatus{State: corev1.ContainerState{Terminated: failed}}), want: true, wantMsg: "[config-migrate] ERROR: [Errno 30] Read-only file system"},
		{
			name: "crashloop: waiting with failed last termination",
			pod: seedPod(corev1.ContainerStatus{
				State:                corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
				LastTerminationState: corev1.ContainerState{Terminated: failed},
			}),
			want: true, wantMsg: "[config-migrate] ERROR: [Errno 30] Read-only file system",
		},
		{
			name: "recovered: completed after an earlier failure",
			pod: seedPod(corev1.ContainerStatus{
				State:                corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0}},
				LastTerminationState: corev1.ContainerState{Terminated: failed},
			}),
			want: false,
		},
		{
			name: "no message falls back to exit code",
			pod:  seedPod(corev1.ContainerStatus{State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 2}}}),
			want: true, wantMsg: "config-seed init container exited with code 2",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, msg := ConfigSeedFailure(tc.pod)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.wantMsg, msg)
		})
	}
}

func TestConfigSeedFailure_TruncatesLongMessages(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("x", 5000) + "TAIL"
	pod := seedPod(corev1.ContainerStatus{State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 1, Message: long}}})
	got, msg := ConfigSeedFailure(pod)
	assert.True(t, got)
	assert.LessOrEqual(t, len(msg), 1024+3)
	assert.True(t, strings.HasSuffix(msg, "TAIL"), "keeps the tail, where the error is")
}

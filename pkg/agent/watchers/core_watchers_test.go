package watchers

import (
	"testing"

	"github.com/emissary-ingress/emissary/v3/pkg/kates"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// if the agent has pods that match the service selector labels, it should
// return those pods in the snapshot
func Test_labelMatching(t *testing.T) {
	t.Parallel()
	svcs := []*kates.Service{
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "ns",
			},
			Spec: kates.ServiceSpec{
				Selector: map[string]string{"label": "matching"},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "ns",
			},
			Spec: kates.ServiceSpec{
				Selector: map[string]string{"label2": "alsomatching", "label3": "yay"},
			},
		},
	}

	testCases := []struct {
		testName string
		pod      *kates.Pod
		svcs     []*kates.Service
		res      bool
	}{
		{
			testName: "1 of 1 labels match",
			svcs:     svcs,
			pod: &kates.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod1",
					Namespace: "ns",
					Labels:    map[string]string{"label": "matching", "tag": "1.0"},
				},
				Status: v1.PodStatus{
					Phase: v1.PodRunning,
				},
			},
			res: true,
		},
		{
			testName: "wrong ns",
			svcs:     svcs,
			pod: &kates.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod1",
					Namespace: "wrong-ns",
					Labels:    map[string]string{"label": "matching", "tag": "1.0"},
				},
				Status: v1.PodStatus{
					Phase: v1.PodRunning,
				},
			},
			res: false,
		},
		{
			testName: "2 of 2 labels match",
			svcs:     svcs,
			pod: &kates.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod2",
					Namespace: "ns",
					Labels:    map[string]string{"label2": "alsomatching", "tag": "1.0", "label3": "yay"},
				},
				Status: v1.PodStatus{
					Phase: v1.PodFailed,
				},
			},
			res: true,
		},
		{
			testName: "1 of 2 labels match",
			svcs:     svcs,
			pod: &kates.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod3",
					Namespace: "ns",
					Labels:    map[string]string{"label2": "alsomatching", "tag": "1.0"},
				},
				Status: v1.PodStatus{
					Phase: v1.PodSucceeded,
				},
			},
			res: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.res, labelMatching(tc.pod, tc.svcs))
		})
	}
}

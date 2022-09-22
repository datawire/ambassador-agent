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
	svcs := []*kates.Service{
		{
			Spec: kates.ServiceSpec{
				Selector: map[string]string{"label": "matching"},
			},
		},
		{
			Spec: kates.ServiceSpec{
				Selector: map[string]string{"label2": "alsomatching", "label3": "yay"},
			},
		},
	}

	pods := []*kates.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod1",
				Namespace: "ns",
				Labels:    map[string]string{"label": "matching", "tag": "1.0"},
			},
			Status: v1.PodStatus{
				Phase: v1.PodRunning,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod2",
				Namespace: "ns",
				Labels:    map[string]string{"label2": "alsomatching", "tag": "1.0", "label3": "yay"},
			},
			Status: v1.PodStatus{
				Phase: v1.PodFailed,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod3",
				Namespace: "ns",
				Labels:    map[string]string{"label2": "alsomatching", "tag": "1.0"},
			},
			Status: v1.PodStatus{
				Phase: v1.PodSucceeded,
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
			pod:      pods[0],
			res:      true,
		},
		{
			testName: "2 of 2 labels match",
			svcs:     svcs,
			pod:      pods[1],
			res:      true,
		},
		{
			testName: "1 of 2 labels match",
			svcs:     svcs,
			pod:      pods[2],
			res:      false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			assert.Equal(t, tc.res, labelMatching(tc.pod, tc.svcs))
		})
	}
}

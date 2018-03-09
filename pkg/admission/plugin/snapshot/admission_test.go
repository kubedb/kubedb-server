package snapshot

import (
	"net/http"
	"os"
	"testing"

	"github.com/appscode/kutil/meta"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	"github.com/kubedb/apimachinery/client/clientset/versioned/scheme"
	"github.com/kubedb/kubedb-server/pkg/admission/util"
	admission "k8s.io/api/admission/v1beta1"
	authenticationV1 "k8s.io/api/authentication/v1"
	core "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clientSetScheme "k8s.io/client-go/kubernetes/scheme"
)

func init() {
	scheme.AddToScheme(clientSetScheme.Scheme)
	os.Setenv(util.EnvSvcAccountName, "kubedb-operator")
	os.Setenv("KUBE_NAMESPACE", "kube-system")
}

var requestKind = metaV1.GroupVersionKind{
	Group:   api.SchemeGroupVersion.Group,
	Version: api.SchemeGroupVersion.Version,
	Kind:    api.ResourceKindSnapshot,
}

func TestSnapshotValidator_Admit(t *testing.T) {
	for _, c := range cases {
		t.Run(c.testName, func(t *testing.T) {
			validator := SnapshotValidator{}

			validator.initialized = true
			validator.client = fake.NewSimpleClientset()

			objJS, err := meta.MarshalToJson(&c.object, api.SchemeGroupVersion)
			if err != nil {
				panic(err)
			}
			oldObjJS, err := meta.MarshalToJson(&c.oldObject, api.SchemeGroupVersion)
			if err != nil {
				panic(err)
			}

			req := new(admission.AdmissionRequest)

			req.Kind = c.kind
			req.Name = c.objectName
			req.Namespace = c.namespace
			req.Operation = c.operation
			req.UserInfo = c.userInfo
			req.Object.Raw = objJS
			req.OldObject.Raw = oldObjJS

			if c.operation == admission.Delete {
				req.Object = runtime.RawExtension{}
			}
			if c.operation != admission.Update {
				req.OldObject = runtime.RawExtension{}
			}

			response := validator.Admit(req)
			if c.result == true {
				if response.Allowed != true {
					t.Errorf("expected: 'Allowed=true'. but got response: %v", response)
				}
			} else if c.result == false {
				if response.Allowed == true || response.Result.Code == http.StatusInternalServerError {
					t.Errorf("expected: 'Allowed=false', but got response: %v", response)
				}
			}
		})
	}

}

var cases = []struct {
	testName   string
	kind       metaV1.GroupVersionKind
	objectName string
	namespace  string
	operation  admission.Operation
	userInfo   authenticationV1.UserInfo
	object     api.Snapshot
	oldObject  api.Snapshot
	heatUp     bool
	result     bool
}{
	{"Create Valid Snapshot By User",
		requestKind,
		"foo",
		"default",
		admission.Create,
		userIsHooman(),
		sampleSnapshot(),
		api.Snapshot{},
		false,
		true,
	},
	{"Create Valid Snapshot By Operator",
		requestKind,
		"foo",
		"default",
		admission.Create,
		userIsHooman(),
		sampleSnapshot(),
		api.Snapshot{},
		false,
		true,
	},
	{"Create Invalid Snapshot By User",
		requestKind,
		"foo",
		"default",
		admission.Create,
		userIsOperator(),
		getAwkwardSnapshot(),
		api.Snapshot{},
		false,
		false,
	},
	{"Create Invalid Snapshot By Operator",
		requestKind,
		"foo",
		"default",
		admission.Create,
		userIsHooman(),
		getAwkwardSnapshot(),
		api.Snapshot{},
		false,
		false,
	},
	{"Edit Status By User",
		requestKind,
		"foo",
		"default",
		admission.Update,
		userIsHooman(),
		editStatus(sampleSnapshot()),
		sampleSnapshot(),
		false,
		false,
	},
	{"Edit Status By Operator",
		requestKind,
		"foo",
		"default",
		admission.Update,
		userIsOperator(),
		editStatus(sampleSnapshot()),
		sampleSnapshot(),
		false,
		true,
	},
	{"Delete Snapshot by Operator",
		requestKind,
		"foo",
		"default",
		admission.Delete,
		userIsOperator(),
		sampleSnapshot(),
		api.Snapshot{},
		false,
		true,
	},
	{"Delete Snapshot when by User",
		requestKind,
		"foo",
		"default",
		admission.Delete,
		userIsHooman(),
		sampleSnapshot(),
		api.Snapshot{},
		false,
		true,
	},
	{"Delete Non Existing Snapshot By Operator",
		requestKind,
		"foo",
		"default",
		admission.Delete,
		userIsOperator(),
		api.Snapshot{},
		api.Snapshot{},
		false,
		true,
	},
	{"Delete Non Existing Snapshot By User",
		requestKind,
		"foo",
		"default",
		admission.Delete,
		userIsHooman(),
		api.Snapshot{},
		api.Snapshot{},
		false,
		true,
	},
}

func sampleSnapshot() api.Snapshot {
	return api.Snapshot{
		TypeMeta: metaV1.TypeMeta{
			Kind:       api.ResourceKindSnapshot,
			APIVersion: api.SchemeGroupVersion.String(),
		},
		ObjectMeta: metaV1.ObjectMeta{
			Name:      "foo",
			Namespace: "default",
			Labels: map[string]string{
				api.LabelDatabaseKind: api.ResourceKindSnapshot,
			},
		},
		Spec: api.SnapshotSpec{
			SnapshotStorageSpec: api.SnapshotStorageSpec{
				Local: &api.LocalSpec{
					MountPath: "/repo",
					VolumeSource: core.VolumeSource{
						EmptyDir: &core.EmptyDirVolumeSource{},
					},
				},
			},
		},
	}
}

func getAwkwardSnapshot() api.Snapshot {
	redis := sampleSnapshot()
	redis.Spec =api.SnapshotSpec{
		DatabaseName: "foo",
		SnapshotStorageSpec: api.SnapshotStorageSpec{
			StorageSecretName: "foo-secret",
			GCS: &api.GCSSpec{
				Bucket: "bar",
			},
		},
	}
	return redis
}

func editStatus(old api.Snapshot) api.Snapshot {
	old.Status = api.SnapshotStatus{
		Phase: api.SnapshotPhaseRunning,
	}
	return old
}

func userIsOperator() authenticationV1.UserInfo {
	return authenticationV1.UserInfo{
		Username: "system:serviceaccount:kube-system:kubedb-operator",
		Groups: []string{
			"system:serviceaccounts",
			"system:serviceaccounts:kube-system",
			"system:authenticated",
		},
	}
}

func userIsHooman() authenticationV1.UserInfo {
	return authenticationV1.UserInfo{
		Username: "minikube-user",
		Groups: []string{
			"system:masters",
			"system:authenticated",
		},
	}
}

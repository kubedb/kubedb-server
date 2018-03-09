package dormant_database

import (
	"net/http"
	"os"
	"testing"

	"github.com/appscode/kutil/meta"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	ext_fake "github.com/kubedb/apimachinery/client/clientset/versioned/fake"
	"github.com/kubedb/apimachinery/client/clientset/versioned/scheme"
	"github.com/kubedb/kubedb-server/pkg/admission/util"
	admission "k8s.io/api/admission/v1beta1"
	authenticationv1 "k8s.io/api/authentication/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
)

func init() {
	scheme.AddToScheme(clientsetscheme.Scheme)
	os.Setenv(util.EnvSvcAccountName, "kubedb-operator")
	os.Setenv("KUBE_NAMESPACE", "kube-system")
}

var requestKind = metav1.GroupVersionKind{
	Group:   api.SchemeGroupVersion.Group,
	Version: api.SchemeGroupVersion.Version,
	Kind:    api.ResourceKindDormantDatabase,
}

func TestDormantDatabaseValidator_Admit(t *testing.T) {
	for _, c := range cases {
		t.Run(c.testName, func(t *testing.T) {
			validator := DormantDatabaseValidator{}
			validator.initialized = true
			validator.client = fake.NewSimpleClientset()
			validator.extClient = ext_fake.NewSimpleClientset()

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

			if c.heatUp {
				if _, err := validator.extClient.KubedbV1alpha1().DormantDatabases(c.namespace).Create(&c.object); err != nil && !kerr.IsAlreadyExists(err) {
					t.Errorf(err.Error())
				}
			}
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
	kind       metav1.GroupVersionKind
	objectName string
	namespace  string
	operation  admission.Operation
	userInfo   authenticationv1.UserInfo
	object     api.DormantDatabase
	oldObject  api.DormantDatabase
	heatUp     bool
	result     bool
}{
	{"Create Dormant Database By Operator",
		requestKind,
		"foo",
		"default",
		admission.Create,
		userIsOperator(),
		sampleDormantDatabase(),
		api.DormantDatabase{},
		false,
		true,
	},
	{"Create Dormant Database By User",
		requestKind,
		"foo",
		"default",
		admission.Create,
		userIsHooman(),
		sampleDormantDatabase(),
		api.DormantDatabase{},
		false,
		false,
	},
	{"Edit Status By Operator",
		requestKind,
		"foo",
		"default",
		admission.Update,
		userIsOperator(),
		editStatus(sampleDormantDatabase()),
		sampleDormantDatabase(),
		false,
		true,
	},
	{"Edit Status By User",
		requestKind,
		"foo",
		"default",
		admission.Update,
		userIsHooman(),
		editStatus(sampleDormantDatabase()),
		sampleDormantDatabase(),
		false,
		false,
	},
	{"Edit Spec.Origin By User",
		requestKind,
		"foo",
		"default",
		admission.Update,
		userIsHooman(),
		editSpecOrigin(sampleDormantDatabase()),
		sampleDormantDatabase(),
		false,
		false,
	},
	{"Edit Spec.Resume By Operator",
		requestKind,
		"foo",
		"default",
		admission.Update,
		userIsOperator(),
		editSpecResume(sampleDormantDatabase()),
		sampleDormantDatabase(),
		false,
		true,
	},
	{"Edit Spec.Resume By User",
		requestKind,
		"foo",
		"default",
		admission.Update,
		userIsHooman(),
		editSpecResume(sampleDormantDatabase()),
		sampleDormantDatabase(),
		false,
		true,
	},
	{"Edit Spec.WipeOut By Operator",
		requestKind,
		"foo",
		"default",
		admission.Update,
		userIsOperator(),
		editSpecWipeOut(sampleDormantDatabase()),
		sampleDormantDatabase(),
		false,
		true,
	},
	{"Edit Spec.WipeOut By User",
		requestKind,
		"foo",
		"default",
		admission.Update,
		userIsHooman(),
		editSpecWipeOut(sampleDormantDatabase()),
		sampleDormantDatabase(),
		false,
		true,
	},
	{"Delete Without Wiping By Operator",
		requestKind,
		"foo",
		"default",
		admission.Delete,
		userIsOperator(),
		sampleDormantDatabase(),
		api.DormantDatabase{},
		true,
		true,
	},
	{"Delete Without Wiping By User",
		requestKind,
		"foo",
		"default",
		admission.Delete,
		userIsHooman(),
		sampleDormantDatabase(),
		api.DormantDatabase{},
		true,
		false,
	},
	{"Delete With Wiping By User",
		requestKind,
		"foo",
		"default",
		admission.Delete,
		userIsHooman(),
		editStatusWipedOut(sampleDormantDatabase()),
		api.DormantDatabase{},
		true,
		true,
	},
	{"Delete Non Existing Dormant By Operator",
		requestKind,
		"foo",
		"default",
		admission.Delete,
		userIsOperator(),
		api.DormantDatabase{},
		api.DormantDatabase{},
		false,
		true,
	},
	{"Delete Non Existing Dormant By User",
		requestKind,
		"foo",
		"default",
		admission.Delete,
		userIsHooman(),
		api.DormantDatabase{},
		api.DormantDatabase{},
		false,
		true,
	},
}

func sampleDormantDatabase() api.DormantDatabase {
	return api.DormantDatabase{
		TypeMeta: metav1.TypeMeta{
			Kind:       api.ResourceKindDormantDatabase,
			APIVersion: api.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "default",
			Labels: map[string]string{
				api.LabelDatabaseKind: api.ResourceKindMongoDB,
			},
		},
		Spec: api.DormantDatabaseSpec{
			Origin: api.Origin{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
					Labels: map[string]string{
						api.LabelDatabaseKind: api.ResourceKindMongoDB,
					},
					Annotations: map[string]string{
						api.AnnotationInitialized: "",
					},
				},
				Spec: api.OriginSpec{
					MongoDB: &api.MongoDBSpec{},
				},
			},
		},
	}
}

func editSpecOrigin(old api.DormantDatabase) api.DormantDatabase {
	old.Spec.Origin.Annotations = nil
	return old
}

func editStatus(old api.DormantDatabase) api.DormantDatabase {
	old.Status = api.DormantDatabaseStatus{
		Phase: api.DormantDatabasePhasePaused,
	}
	return old
}

func editSpecWipeOut(old api.DormantDatabase) api.DormantDatabase {
	old.Spec.WipeOut = true
	return old
}

func editSpecResume(old api.DormantDatabase) api.DormantDatabase {
	old.Spec.Resume = true
	return old
}

func editStatusWipedOut(old api.DormantDatabase) api.DormantDatabase {
	old.Spec.WipeOut = true
	old.Status.Phase = api.DormantDatabasePhaseWipedOut
	return old
}

func userIsOperator() authenticationv1.UserInfo {
	return authenticationv1.UserInfo{
		Username: "system:serviceaccount:kube-system:kubedb-operator",
		Groups: []string{
			"system:serviceaccounts",
			"system:serviceaccounts:kube-system",
			"system:authenticated",
		},
	}
}

func userIsHooman() authenticationv1.UserInfo {
	return authenticationv1.UserInfo{
		Username: "minikube-user",
		Groups: []string{
			"system:masters",
			"system:authenticated",
		},
	}
}

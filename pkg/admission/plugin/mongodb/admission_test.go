package mongodb

import (
	"net/http"
	"os"
	"testing"

	"github.com/appscode/go/types"
	kubeMon "github.com/appscode/kube-mon/api"
	"github.com/appscode/kutil/meta"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	extFake "github.com/kubedb/apimachinery/client/clientset/versioned/fake"
	"github.com/kubedb/apimachinery/client/clientset/versioned/scheme"
	"github.com/kubedb/kubedb-server/pkg/admission/util"
	admission "k8s.io/api/admission/v1beta1"
	authenticationV1 "k8s.io/api/authentication/v1"
	core "k8s.io/api/core/v1"
	storageV1beta1 "k8s.io/api/storage/v1beta1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
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
	Kind:    api.ResourceKindMongoDB,
}

func TestMongoDBValidator_Admit(t *testing.T) {
	for _, c := range cases {
		t.Run(c.testName, func(t *testing.T) {
			validator := MongoDBValidator{}

			validator.initialized = true
			validator.extClient = extFake.NewSimpleClientset()
			validator.client = fake.NewSimpleClientset(
				&core.Secret{
					ObjectMeta: metaV1.ObjectMeta{
						Name:      "foo-auth",
						Namespace: "default",
					},
				},
				&storageV1beta1.StorageClass{
					ObjectMeta: metaV1.ObjectMeta{
						Name: "standard",
					},
				},
			)

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
				if _, err := validator.extClient.KubedbV1alpha1().MongoDBs(c.namespace).Create(&c.object); err != nil && !kerr.IsAlreadyExists(err) {
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
	kind       metaV1.GroupVersionKind
	objectName string
	namespace  string
	operation  admission.Operation
	userInfo   authenticationV1.UserInfo
	object     api.MongoDB
	oldObject  api.MongoDB
	heatUp     bool
	result     bool
}{
	{"Create Valid MongoDB By User",
		requestKind,
		"foo",
		"default",
		admission.Create,
		userIsHooman(),
		sampleMongoDB(),
		api.MongoDB{},
		false,
		true,
	},
	{"Create Invalid MongoDB By User",
		requestKind,
		"foo",
		"default",
		admission.Create,
		userIsHooman(),
		getAwkwardMongoDB(),
		api.MongoDB{},
		false,
		false,
	},
	{"Create Invalid MongoDB By Operator",
		requestKind,
		"foo",
		"default",
		admission.Create,
		userIsHooman(),
		getAwkwardMongoDB(),
		api.MongoDB{},
		false,
		false,
	},
	{"Edit MongoDB Spec.DatabaseSecret By User",
		requestKind,
		"foo",
		"default",
		admission.Update,
		userIsHooman(),
		editSpecSecret(sampleMongoDB()),
		sampleMongoDB(),
		false,
		false,
	},
	{"Edit MongoDB Spec.DatabaseSecret By Operator",
		requestKind,
		"foo",
		"default",
		admission.Update,
		userIsOperator(),
		editSpecSecret(sampleMongoDB()),
		sampleMongoDB(),
		false,
		true,
	},
	{"Edit Status By User",
		requestKind,
		"foo",
		"default",
		admission.Update,
		userIsHooman(),
		editStatus(sampleMongoDB()),
		sampleMongoDB(),
		false,
		false,
	},
	{"Edit Status By Operator",
		requestKind,
		"foo",
		"default",
		admission.Update,
		userIsOperator(),
		editStatus(sampleMongoDB()),
		sampleMongoDB(),
		false,
		true,
	},
	{"Edit Spec.Monitor By User",
		requestKind,
		"foo",
		"default",
		admission.Update,
		userIsHooman(),
		editSpecMonitor(sampleMongoDB()),
		sampleMongoDB(),
		false,
		true,
	},
	{"Edit Spec.Monitor By Operator",
		requestKind,
		"foo",
		"default",
		admission.Update,
		userIsOperator(),
		editSpecMonitor(sampleMongoDB()),
		sampleMongoDB(),
		false,
		true,
	},
	{"Edit Invalid Spec.Monitor By User",
		requestKind,
		"foo",
		"default",
		admission.Update,
		userIsHooman(),
		editSpecInvalidMonitor(sampleMongoDB()),
		sampleMongoDB(),
		false,
		false,
	},
	{"Edit Invalid Spec.Monitor By Operator",
		requestKind,
		"foo",
		"default",
		admission.Update,
		userIsOperator(),
		editSpecInvalidMonitor(sampleMongoDB()),
		sampleMongoDB(),
		false,
		false,
	},
	{"Edit Spec.DoNotPause By User",
		requestKind,
		"foo",
		"default",
		admission.Update,
		userIsHooman(),
		editSpecDoNotPause(sampleMongoDB()),
		sampleMongoDB(),
		false,
		true,
	},
	{"Delete Mongodb when Spec.DoNotPause=true by Operator",
		requestKind,
		"foo",
		"default",
		admission.Delete,
		userIsOperator(),
		sampleMongoDB(),
		api.MongoDB{},
		true,
		false,
	},
	{"Delete Mongodb when Spec.DoNotPause=true by User",
		requestKind,
		"foo",
		"default",
		admission.Delete,
		userIsHooman(),
		sampleMongoDB(),
		api.MongoDB{},
		true,
		false,
	},
	{"Delete Mongodb when Spec.DoNotPause=false by Operator",
		requestKind,
		"foo",
		"default",
		admission.Delete,
		userIsOperator(),
		editSpecDoNotPause(sampleMongoDB()),
		api.MongoDB{},
		true,
		true,
	},
	{"Delete Mongodb when Spec.DoNotPause=false by User",
		requestKind,
		"foo",
		"default",
		admission.Delete,
		userIsHooman(),
		editSpecDoNotPause(sampleMongoDB()),
		api.MongoDB{},
		true,
		true,
	},
	{"Delete Non Existing MongoDB By Operator",
		requestKind,
		"foo",
		"default",
		admission.Delete,
		userIsOperator(),
		api.MongoDB{},
		api.MongoDB{},
		false,
		true,
	},
	{"Delete Non Existing MongoDB By User",
		requestKind,
		"foo",
		"default",
		admission.Delete,
		userIsHooman(),
		api.MongoDB{},
		api.MongoDB{},
		false,
		true,
	},
}

func sampleMongoDB() api.MongoDB {
	return api.MongoDB{
		TypeMeta: metaV1.TypeMeta{
			Kind:       api.ResourceKindMongoDB,
			APIVersion: api.SchemeGroupVersion.String(),
		},
		ObjectMeta: metaV1.ObjectMeta{
			Name:      "foo",
			Namespace: "default",
			Labels: map[string]string{
				api.LabelDatabaseKind: api.ResourceKindMongoDB,
			},
		},
		Spec: api.MongoDBSpec{
			Version:    "3.4",
			DoNotPause: true,
			Storage: &core.PersistentVolumeClaimSpec{
				StorageClassName: types.StringP("standard"),
				Resources: core.ResourceRequirements{
					Requests: core.ResourceList{
						core.ResourceStorage: resource.MustParse("100Mi"),
					},
				},
			},
			Init: &api.InitSpec{
				ScriptSource: &api.ScriptSourceSpec{
					VolumeSource: core.VolumeSource{
						GitRepo: &core.GitRepoVolumeSource{
							Repository: "https://github.com/kubedb/mongodb-init-scripts.git",
							Directory:  ".",
						},
					},
				},
			},
		},
	}
}

func getAwkwardMongoDB() api.MongoDB {
	mongodb := sampleMongoDB()
	mongodb.Spec.Version = "3.0"
	return mongodb
}

func editSpecSecret(old api.MongoDB) api.MongoDB {
	old.Spec.DatabaseSecret = &core.SecretVolumeSource{
		SecretName: "foo-auth",
	}
	return old
}

func editStatus(old api.MongoDB) api.MongoDB {
	old.Status = api.MongoDBStatus{
		Phase: api.DatabasePhaseCreating,
	}
	return old
}

func editSpecMonitor(old api.MongoDB) api.MongoDB {
	old.Spec.Monitor = &kubeMon.AgentSpec{
		Agent: kubeMon.AgentPrometheusBuiltin,
	}
	return old
}

// should be failed because more fields required for COreOS Monitoring
func editSpecInvalidMonitor(old api.MongoDB) api.MongoDB {
	old.Spec.Monitor = &kubeMon.AgentSpec{
		Agent: kubeMon.AgentCoreOSPrometheus,
	}
	return old
}

func editSpecDoNotPause(old api.MongoDB) api.MongoDB {
	old.Spec.DoNotPause = false
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

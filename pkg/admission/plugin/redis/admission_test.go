package redis

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
	Kind:    api.ResourceKindRedis,
}

func TestRedisValidator_Admit(t *testing.T) {
	for _, c := range cases {
		t.Run(c.testName, func(t *testing.T) {
			validator := RedisValidator{}

			validator.initialized = true
			validator.extClient = extFake.NewSimpleClientset()
			validator.client = fake.NewSimpleClientset(
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
				if _, err := validator.extClient.KubedbV1alpha1().Redises(c.namespace).Create(&c.object); err != nil && !kerr.IsAlreadyExists(err) {
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
	object     api.Redis
	oldObject  api.Redis
	heatUp     bool
	result     bool
}{
	{"Create Valid Redis By User",
		requestKind,
		"foo",
		"default",
		admission.Create,
		userIsHooman(),
		sampleRedis(),
		api.Redis{},
		false,
		true,
	},
	{"Create Invalid Redis By User",
		requestKind,
		"foo",
		"default",
		admission.Create,
		userIsHooman(),
		getAwkwardRedis(),
		api.Redis{},
		false,
		false,
	},
	{"Create Invalid Redis By Operator",
		requestKind,
		"foo",
		"default",
		admission.Create,
		userIsHooman(),
		getAwkwardRedis(),
		api.Redis{},
		false,
		false,
	},
	{"Edit Redis Spec.Version By User",
		requestKind,
		"foo",
		"default",
		admission.Update,
		userIsHooman(),
		editSpecVersion(sampleRedis()),
		sampleRedis(),
		false,
		false,
	},
	{"Edit Status By User",
		requestKind,
		"foo",
		"default",
		admission.Update,
		userIsHooman(),
		editStatus(sampleRedis()),
		sampleRedis(),
		false,
		false,
	},
	{"Edit Status By Operator",
		requestKind,
		"foo",
		"default",
		admission.Update,
		userIsOperator(),
		editStatus(sampleRedis()),
		sampleRedis(),
		false,
		true,
	},
	{"Edit Spec.Monitor By User",
		requestKind,
		"foo",
		"default",
		admission.Update,
		userIsHooman(),
		editSpecMonitor(sampleRedis()),
		sampleRedis(),
		false,
		true,
	},
	{"Edit Spec.Monitor By Operator",
		requestKind,
		"foo",
		"default",
		admission.Update,
		userIsOperator(),
		editSpecMonitor(sampleRedis()),
		sampleRedis(),
		false,
		true,
	},
	{"Edit Invalid Spec.Monitor By User",
		requestKind,
		"foo",
		"default",
		admission.Update,
		userIsHooman(),
		editSpecInvalidMonitor(sampleRedis()),
		sampleRedis(),
		false,
		false,
	},
	{"Edit Invalid Spec.Monitor By Operator",
		requestKind,
		"foo",
		"default",
		admission.Update,
		userIsOperator(),
		editSpecInvalidMonitor(sampleRedis()),
		sampleRedis(),
		false,
		false,
	},
	{"Edit Spec.DoNotPause By User",
		requestKind,
		"foo",
		"default",
		admission.Update,
		userIsHooman(),
		editSpecDoNotPause(sampleRedis()),
		sampleRedis(),
		false,
		true,
	},
	{"Delete Redis when Spec.DoNotPause=true by Operator",
		requestKind,
		"foo",
		"default",
		admission.Delete,
		userIsOperator(),
		sampleRedis(),
		api.Redis{},
		true,
		false,
	},
	{"Delete Redis when Spec.DoNotPause=true by User",
		requestKind,
		"foo",
		"default",
		admission.Delete,
		userIsHooman(),
		sampleRedis(),
		api.Redis{},
		true,
		false,
	},
	{"Delete Redis when Spec.DoNotPause=false by Operator",
		requestKind,
		"foo",
		"default",
		admission.Delete,
		userIsOperator(),
		editSpecDoNotPause(sampleRedis()),
		api.Redis{},
		true,
		true,
	},
	{"Delete Redis when Spec.DoNotPause=false by User",
		requestKind,
		"foo",
		"default",
		admission.Delete,
		userIsHooman(),
		editSpecDoNotPause(sampleRedis()),
		api.Redis{},
		true,
		true,
	},
	{"Delete Non Existing Redis By Operator",
		requestKind,
		"foo",
		"default",
		admission.Delete,
		userIsOperator(),
		api.Redis{},
		api.Redis{},
		false,
		true,
	},
	{"Delete Non Existing Redis By User",
		requestKind,
		"foo",
		"default",
		admission.Delete,
		userIsHooman(),
		api.Redis{},
		api.Redis{},
		false,
		true,
	},
}

func sampleRedis() api.Redis {
	return api.Redis{
		TypeMeta: metaV1.TypeMeta{
			Kind:       api.ResourceKindRedis,
			APIVersion: api.SchemeGroupVersion.String(),
		},
		ObjectMeta: metaV1.ObjectMeta{
			Name:      "foo",
			Namespace: "default",
			Labels: map[string]string{
				api.LabelDatabaseKind: api.ResourceKindRedis,
			},
		},
		Spec: api.RedisSpec{
			Version:    "4",
			DoNotPause: true,
			Storage: &core.PersistentVolumeClaimSpec{
				StorageClassName: types.StringP("standard"),
				Resources: core.ResourceRequirements{
					Requests: core.ResourceList{
						core.ResourceStorage: resource.MustParse("100Mi"),
					},
				},
			},
		},
	}
}

func getAwkwardRedis() api.Redis {
	redis := sampleRedis()
	redis.Spec.Version = "3.0"
	return redis
}

func editSpecVersion(old api.Redis) api.Redis {
	old.Spec.Version = "4.4"
	return old
}

func editStatus(old api.Redis) api.Redis {
	old.Status = api.RedisStatus{
		Phase: api.DatabasePhaseCreating,
	}
	return old
}

func editSpecMonitor(old api.Redis) api.Redis {
	old.Spec.Monitor = &kubeMon.AgentSpec{
		Agent: kubeMon.AgentPrometheusBuiltin,
	}
	return old
}

// should be failed because more fields required for COreOS Monitoring
func editSpecInvalidMonitor(old api.Redis) api.Redis {
	old.Spec.Monitor = &kubeMon.AgentSpec{
		Agent: kubeMon.AgentCoreOSPrometheus,
	}
	return old
}

func editSpecDoNotPause(old api.Redis) api.Redis {
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

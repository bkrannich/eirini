package stset

import (
	"fmt"

	"code.cloudfoundry.org/eirini/k8s/shared"
	"code.cloudfoundry.org/eirini/k8s/utils"
	"code.cloudfoundry.org/eirini/k8s/utils/dockerutils"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/lager"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//counterfeiter:generate . SecretsCreator
//counterfeiter:generate . StatefulSetCreator
//counterfeiter:generate . LRPToStatefulSet

type LRPToStatefulSet func(statefulSetName string, lrp *opi.LRP) (*appsv1.StatefulSet, error)

type SecretsCreator interface {
	Create(namespace string, secret *corev1.Secret) (*corev1.Secret, error)
}

type StatefulSetCreator interface {
	Create(namespace string, statefulSet *appsv1.StatefulSet) (*appsv1.StatefulSet, error)
}

type Desirer struct {
	logger                    lager.Logger
	secrets                   SecretsCreator
	statefulSets              StatefulSetCreator
	lrpToStatefulSet          LRPToStatefulSet
	createPodDisruptionBudget createPodDisruptionBudgetFunc
}

func NewDesirer(
	logger lager.Logger,
	secrets SecretsCreator,
	statefulSets StatefulSetCreator,
	lrpToStatefulSet LRPToStatefulSet,
	podDisruptionBudget PodDisruptionBudgetCreator,
) Desirer {
	return Desirer{
		logger:                    logger,
		secrets:                   secrets,
		statefulSets:              statefulSets,
		lrpToStatefulSet:          lrpToStatefulSet,
		createPodDisruptionBudget: newCreatePodDisruptionBudgetFunc(podDisruptionBudget),
	}
}

func (d *Desirer) Desire(namespace string, lrp *opi.LRP, opts ...shared.Option) error {
	logger := d.logger.Session("desire", lager.Data{"guid": lrp.GUID, "version": lrp.Version, "namespace": namespace})

	statefulSetName, err := utils.GetStatefulsetName(lrp)
	if err != nil {
		return err
	}

	if lrp.PrivateRegistry != nil {
		err = d.createRegistryCredsSecret(namespace, statefulSetName, lrp)
		if err != nil {
			return err
		}
	}

	st, err := d.lrpToStatefulSet(statefulSetName, lrp)
	if err != nil {
		return err
	}

	st.Namespace = namespace

	err = shared.ApplyOpts(st, opts...)
	if err != nil {
		return err
	}

	if _, err := d.statefulSets.Create(namespace, st); err != nil {
		var statusErr *k8serrors.StatusError
		if errors.As(err, &statusErr) && statusErr.Status().Reason == metav1.StatusReasonAlreadyExists {
			logger.Debug("statefulset-already-exists", lager.Data{"error": err.Error()})

			return nil
		}

		return errors.Wrap(err, "failed to create statefulset")
	}

	if err := d.createPodDisruptionBudget(namespace, statefulSetName, lrp); err != nil {
		logger.Error("failed-to-create-pod-disruption-budget", err)

		return errors.Wrap(err, "failed to create pod disruption budget")
	}

	return nil
}

func (d *Desirer) createRegistryCredsSecret(namespace, statefulSetName string, lrp *opi.LRP) error {
	secret, err := generateRegistryCredsSecret(statefulSetName, lrp)
	if err != nil {
		return errors.Wrap(err, "failed to generate private registry secret for statefulset")
	}

	_, err = d.secrets.Create(namespace, secret)

	return errors.Wrap(err, "failed to create private registry secret for statefulset")
}

func generateRegistryCredsSecret(statefulSetName string, lrp *opi.LRP) (*corev1.Secret, error) {
	dockerConfig := dockerutils.NewDockerConfig(
		lrp.PrivateRegistry.Server,
		lrp.PrivateRegistry.Username,
		lrp.PrivateRegistry.Password,
	)

	dockerConfigJSON, err := dockerConfig.JSON()
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate privete registry config")
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: privateRegistrySecretName(statefulSetName),
		},
		Type: corev1.SecretTypeDockerConfigJson,
		StringData: map[string]string{
			dockerutils.DockerConfigKey: dockerConfigJSON,
		},
	}, nil
}

func privateRegistrySecretName(statefulSetName string) string {
	return fmt.Sprintf("%s-registry-credentials", statefulSetName)
}

package jobs

import (
	"fmt"
	"strings"

	"code.cloudfoundry.org/lager"
	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
)

//counterfeiter:generate . JobDeleter
//counterfeiter:generate . SecretDeleter

type JobDeleter interface {
	Delete(namespace string, name string) error
}

type SecretDeleter interface {
	Delete(namespace, name string) error
}

type Deleter struct {
	logger        lager.Logger
	jobGetter     JobGetter
	jobDeleter    JobDeleter
	secretDeleter SecretDeleter
}

func NewDeleter(
	logger lager.Logger,
	jobGetter JobGetter,
	jobDeleter JobDeleter,
	secretDeleter SecretDeleter,
) Deleter {
	return Deleter{
		logger:        logger,
		jobGetter:     jobGetter,
		jobDeleter:    jobDeleter,
		secretDeleter: secretDeleter,
	}
}

func (d *Deleter) Delete(guid string) (string, error) {
	logger := d.logger.Session("delete", lager.Data{"guid": guid})

	job, err := d.getJobByGUID(logger, guid)
	if err != nil {
		return "", err
	}

	return d.delete(logger, job)
}

func (d *Deleter) getJobByGUID(logger lager.Logger, guid string) (batchv1.Job, error) {
	jobs, err := d.jobGetter.GetByGUID(guid, true)
	if err != nil {
		logger.Error("failed-to-list-jobs", err)

		return batchv1.Job{}, errors.Wrap(err, "failed to list jobs")
	}

	if len(jobs) != 1 {
		logger.Error("job-does-not-have-1-instance", nil, lager.Data{"instances": len(jobs)})

		return batchv1.Job{}, fmt.Errorf("job with guid %s should have 1 instance, but it has: %d", guid, len(jobs))
	}

	return jobs[0], nil
}

func (d *Deleter) delete(logger lager.Logger, job batchv1.Job) (string, error) {
	if err := d.deleteDockerRegistrySecret(logger, job); err != nil {
		return "", err
	}

	callbackURL := job.Annotations[AnnotationCompletionCallback]

	if len(job.OwnerReferences) != 0 {
		return callbackURL, nil
	}

	if err := d.jobDeleter.Delete(job.Namespace, job.Name); err != nil {
		logger.Error("failed-to-delete-job", err)

		return "", errors.Wrap(err, "failed to delete job")
	}

	return callbackURL, nil
}

func (d *Deleter) deleteDockerRegistrySecret(logger lager.Logger, job batchv1.Job) error {
	dockerSecretNamePrefix := dockerImagePullSecretNamePrefix(
		job.Annotations[AnnotationAppName],
		job.Annotations[AnnotationSpaceName],
		job.Labels[LabelGUID],
	)

	for _, secret := range job.Spec.Template.Spec.ImagePullSecrets {
		if !strings.HasPrefix(secret.Name, dockerSecretNamePrefix) {
			continue
		}

		if err := d.secretDeleter.Delete(job.Namespace, secret.Name); err != nil {
			logger.Error("failed-to-delete-secret", err, lager.Data{"name": secret.Name, "namespace": job.Namespace})

			return errors.Wrap(err, "failed to delete secret")
		}
	}

	return nil
}

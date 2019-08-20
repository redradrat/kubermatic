package reconciling

import (
	appsv1 "k8s.io/api/apps/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilpointer "k8s.io/utils/pointer"
)

// OwnerRefWrapper is responsible for wrapping a ObjectCreator function, solely to set the OwnerReference to the cluster object
func OwnerRefWrapper(ref metav1.OwnerReference) ObjectModifier {
	return func(create ObjectCreator) ObjectCreator {
		return func(existing runtime.Object) (runtime.Object, error) {
			obj, err := create(existing)
			if err != nil {
				return obj, err
			}

			obj.(metav1.Object).SetOwnerReferences([]metav1.OwnerReference{ref})
			return obj, nil
		}
	}
}

// DefaultContainer defaults all Container attributes to the same values as they would get from the Kubernetes API
func DefaultContainer(c *corev1.Container, procMountType *corev1.ProcMountType) {
	if c.ImagePullPolicy == "" {
		c.ImagePullPolicy = corev1.PullIfNotPresent
	}
	if c.TerminationMessagePath == "" {
		c.TerminationMessagePath = corev1.TerminationMessagePathDefault
	}
	if c.TerminationMessagePolicy == "" {
		c.TerminationMessagePolicy = corev1.TerminationMessageReadFile
	}

	for idx := range c.Env {
		if c.Env[idx].ValueFrom != nil && c.Env[idx].ValueFrom.FieldRef != nil {
			if c.Env[idx].ValueFrom.FieldRef.APIVersion == "" {
				c.Env[idx].ValueFrom.FieldRef.APIVersion = "v1"
			}
		}
	}

	// This attribut was added in 1.12
	if c.SecurityContext != nil {
		c.SecurityContext.ProcMount = procMountType
	}
}

// DefaultPodSpec defaults all Container attributes to the same values as they would get from the Kubernetes API
func DefaultPodSpec(old, new corev1.PodSpec) (corev1.PodSpec, error) {
	// make sure to keep the old procmount types in case a creator overrides the entire PodSpec
	initContainerProcMountType := map[string]*corev1.ProcMountType{}
	containerProcMountType := map[string]*corev1.ProcMountType{}
	for _, container := range old.InitContainers {
		if container.SecurityContext != nil {
			initContainerProcMountType[container.Name] = container.SecurityContext.ProcMount
		}
	}
	for _, container := range old.Containers {
		if container.SecurityContext != nil {
			containerProcMountType[container.Name] = container.SecurityContext.ProcMount
		}
	}

	for idx, container := range new.InitContainers {
		DefaultContainer(&new.InitContainers[idx], initContainerProcMountType[container.Name])
	}

	for idx, container := range new.Containers {
		DefaultContainer(&new.Containers[idx], containerProcMountType[container.Name])
	}

	for idx, vol := range new.Volumes {
		if vol.VolumeSource.Secret != nil && vol.VolumeSource.Secret.DefaultMode == nil {
			new.Volumes[idx].Secret.DefaultMode = utilpointer.Int32Ptr(corev1.SecretVolumeSourceDefaultMode)
		}
		if vol.VolumeSource.ConfigMap != nil && vol.VolumeSource.ConfigMap.DefaultMode == nil {
			new.Volumes[idx].ConfigMap.DefaultMode = utilpointer.Int32Ptr(corev1.ConfigMapVolumeSourceDefaultMode)
		}
	}

	return new, nil
}

// DefaultDeployment defaults all Deployment attributes to the same values as they would get from the Kubernetes API
func DefaultDeployment(creator DeploymentCreator) DeploymentCreator {
	return func(d *appsv1.Deployment) (*appsv1.Deployment, error) {
		old := d.DeepCopy()

		d, err := creator(d)
		if err != nil {
			return nil, err
		}

		if d.Spec.Strategy.Type == "" {
			d.Spec.Strategy.Type = appsv1.RollingUpdateDeploymentStrategyType

			if d.Spec.Strategy.RollingUpdate == nil {
				d.Spec.Strategy.RollingUpdate = &appsv1.RollingUpdateDeployment{
					MaxSurge: &intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: 1,
					},
					MaxUnavailable: &intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: 0,
					},
				}
			}
		}

		d.Spec.Template.Spec, err = DefaultPodSpec(old.Spec.Template.Spec, d.Spec.Template.Spec)
		if err != nil {
			return nil, err
		}

		return d, nil
	}
}

// DefaultStatefulSet defaults all StatefulSet attributes to the same values as they would get from the Kubernetes API
func DefaultStatefulSet(creator StatefulSetCreator) StatefulSetCreator {
	return func(ss *appsv1.StatefulSet) (*appsv1.StatefulSet, error) {
		old := ss.DeepCopy()

		ss, err := creator(ss)
		if err != nil {
			return nil, err
		}

		ss.Spec.Template.Spec, err = DefaultPodSpec(old.Spec.Template.Spec, ss.Spec.Template.Spec)
		if err != nil {
			return nil, err
		}

		return ss, nil
	}
}

// DefaultDaemonSet defaults all DaemonSet attributes to the same values as they would get from the Kubernetes API
func DefaultDaemonSet(creator DaemonSetCreator) DaemonSetCreator {
	return func(ds *appsv1.DaemonSet) (*appsv1.DaemonSet, error) {
		old := ds.DeepCopy()

		ds, err := creator(ds)
		if err != nil {
			return nil, err
		}

		ds.Spec.Template.Spec, err = DefaultPodSpec(old.Spec.Template.Spec, ds.Spec.Template.Spec)
		if err != nil {
			return nil, err
		}

		return ds, nil
	}
}

// DefaultCronJob defaults all CronJob attributes to the same values as they would get from the Kubernetes API
func DefaultCronJob(creator CronJobCreator) CronJobCreator {
	return func(cj *batchv1beta1.CronJob) (*batchv1beta1.CronJob, error) {
		old := cj.DeepCopy()

		cj, err := creator(cj)
		if err != nil {
			return nil, err
		}

		cj.Spec.JobTemplate.Spec.Template.Spec, err = DefaultPodSpec(old.Spec.JobTemplate.Spec.Template.Spec, cj.Spec.JobTemplate.Spec.Template.Spec)
		if err != nil {
			return nil, err
		}

		return cj, nil
	}
}

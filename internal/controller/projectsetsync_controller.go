/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"bytes"
	"context"
	"os"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logger "sigs.k8s.io/controller-runtime/pkg/log"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	_ "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	projectv1alpha1 "github.com/migrx-io/projectset-operator/api/v1alpha1"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"

	"gopkg.in/yaml.v2"

	apiyaml "k8s.io/apimachinery/pkg/util/yaml"
)

// Finilizer name
const projectSetSyncFinalizer = "projectsetsync.project.migrx.io/finalizer"

const ERR_TIMEOUT = 10

// ProjectSetSyncReconciler reconciles a ProjectSetSync object
type ProjectSetSyncReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=project.migrx.io,resources=projectsetsyncs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=project.migrx.io,resources=projectsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=project.migrx.io,resources=projectsettemplates,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=project.migrx.io,resources=projectsetsyncs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=project.migrx.io,resources=projectsetsyncs/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ProjectSetSync object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *ProjectSetSyncReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	log = logger.FromContext(ctx).WithValues("projectsetsync", req.NamespacedName)

	//
	// Fetch the instance
	//
	instance := &projectv1alpha1.ProjectSetSync{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// If the custom resource is not found then, it usually means that it was deleted or not created
			// In this way, we will stop the reconciliation
			log.Info("ProjectSetSync resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get ProjectSetSync")
		return ctrl.Result{}, err
	}

	//
	// Set the status as Unknown when no status are available
	//
	if instance.Status.Conditions == nil || len(instance.Status.Conditions) == 0 {

		if err := r.setStatus(ctx, req, instance,
			typeAvailableStatus,
			metav1.ConditionUnknown,
			"Reconciling",
			"Starting reconciliation"); err != nil {

			return ctrl.Result{}, err

		}

	}

	//
	// Add a finalizer. Then, we can define some operations which should
	// occurs before the custom resource to be deleted.
	//
	if !controllerutil.ContainsFinalizer(instance, projectSetSyncFinalizer) {

		log.Info("Adding Finalizer")
		if ok := controllerutil.AddFinalizer(instance, projectSetSyncFinalizer); !ok {
			log.Error(err, "Failed to add finalizer into the custom resource")
			return ctrl.Result{Requeue: true}, nil
		}

		if err := r.Update(ctx, instance); err != nil {
			return ctrl.Result{}, err
		}

	}

	//
	// Check if the instance is marked to be deleted, which is
	// indicated by the deletion timestamp being set.
	//
	isMarkedToBeDeleted := instance.GetDeletionTimestamp() != nil

	log.Info("isMarkedToBeDeleted", "timestamp", instance.GetDeletionTimestamp(), "deletestate", isMarkedToBeDeleted)

	if isMarkedToBeDeleted {
		if controllerutil.ContainsFinalizer(instance, projectSetSyncFinalizer) {

			log.Info("Performing Finalizer Operations before delete CR")

			// Update status
			if err := r.setStatus(ctx, req, instance,
				typeDegradedStatus,
				metav1.ConditionUnknown,
				"Finalizing",
				"Performing finilizer operations"); err != nil {

				return ctrl.Result{}, err

			}

			// Perform all operations required before remove the finalizer and allow
			// the Kubernetes API to remove the custom resource.
			r.doFinalizerOperations(instance)

			// Update status
			if err := r.setStatus(ctx, req, instance,
				typeDegradedStatus,
				metav1.ConditionTrue,
				"Finalizing",
				"Finilizer operations are done"); err != nil {

				return ctrl.Result{}, err

			}

			// Remove finilizers
			log.Info("Removing Finalizer after successfully perform the operations")
			if ok := controllerutil.RemoveFinalizer(instance, projectSetSyncFinalizer); !ok {
				log.Error(err, "Failed to remove finalizer")
				return ctrl.Result{Requeue: true}, nil
			}

			if err := r.Update(ctx, instance); err != nil {
				log.Error(err, "Failed to remove finalizer")
				return ctrl.Result{}, err
			}

			return ctrl.Result{Requeue: true}, nil

		}
	}

	//
	// Git fetch Logic
	//
	log.Info("Fetch data from git",
		"GitRepo", instance.Spec.GitRepo,
		"EnvName", instance.Spec.EnvName,
		"GitBranch", instance.Spec.GitBranch,
		"SyncSecInterval", instance.Spec.SyncSecInterval,
		"ConfFile", instance.Spec.ConfFile,
	)

	localDir := "/tmp/" + instance.Spec.EnvName

	token := os.Getenv("GIT_TOKEN")

	// Check if the directory exists
	if _, err := os.Stat(localDir); os.IsNotExist(err) {
		log.Info("Directory does not existn", "dir", localDir)

		_, err = git.PlainClone(localDir, false, &git.CloneOptions{
			Auth: &http.BasicAuth{
				Username: "projectsetsync",
				Password: token,
			},
			URL:      instance.Spec.GitRepo,
			Progress: os.Stdout,
		})

		if err != nil {
			log.Error(err, "Error cloning repository")
			return ctrl.Result{Requeue: true, RequeueAfter: ERR_TIMEOUT * time.Second}, nil
		}

	} else {
		log.Info("Directory exists", "dir", localDir)

	}

	// Open the repository
	repo, err := git.PlainOpen(localDir)
	if err != nil {
		log.Error(err, "Error opening repository")
		return ctrl.Result{Requeue: true, RequeueAfter: ERR_TIMEOUT * time.Second}, nil
	}

	// Checkout to the desired branch
	worktree, err := repo.Worktree()
	if err != nil {
		log.Error(err, "Error geting worktree")
		return ctrl.Result{Requeue: true, RequeueAfter: ERR_TIMEOUT * time.Second}, nil
	}

	err = worktree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(instance.Spec.GitBranch),
	})
	if err != nil {
		log.Error(err, "Error checking out branch")
		return ctrl.Result{Requeue: true, RequeueAfter: ERR_TIMEOUT * time.Second}, nil
	}

	// Pull last changes
	err = worktree.Pull(&git.PullOptions{})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		log.Error(err, "Error pull out branch")
		return ctrl.Result{Requeue: true, RequeueAfter: ERR_TIMEOUT * time.Second}, nil
	}

	// read repo manifest

	config, err := r.readConfig(ctx, req, instance)
	if err != nil {
		log.Error(err, "Error read config")
		return ctrl.Result{Requeue: true, RequeueAfter: ERR_TIMEOUT * time.Second}, nil
	}

	env := config["envs"][instance.Spec.EnvName]

	templates := env["projectset-templates"]
	crds := env["projectset-crds"]

	log.Info("go to env",
		"templates", templates,
		"crds", crds,
	)

	// check files and apply
	templateFiles, err := r.readCRD(instance, templates)
	if err != nil {
		log.Error(err, "Error read CR")
		return ctrl.Result{Requeue: true, RequeueAfter: ERR_TIMEOUT * time.Second}, nil
	}

	// Apply to cluster
	log.Info("Apply templates", "files", templateFiles)

	// Apply to cluster
	err = r.applyProjectSetTemplate(ctx, req, templateFiles)
	if err != nil {
		log.Error(err, "Error apply projectset CR")
		return ctrl.Result{Requeue: true, RequeueAfter: ERR_TIMEOUT * time.Second}, nil
	}

	// check files and apply
	crdFiles, err := r.readCRD(instance, crds)
	if err != nil {
		log.Error(err, "Error read CR")
		return ctrl.Result{Requeue: true, RequeueAfter: ERR_TIMEOUT * time.Second}, nil
	}

	log.Info("Apply crds", "files", crdFiles)

	// Apply to cluster
	err = r.applyProjectSet(ctx, req, crdFiles)
	if err != nil {
		log.Error(err, "Error apply projectset CR")
		return ctrl.Result{Requeue: true, RequeueAfter: ERR_TIMEOUT * time.Second}, nil
	}

	// update status
	if err := r.setStatus(ctx, req, instance,
		typeAvailableStatus,
		metav1.ConditionTrue,
		"Reconciling",
		"Reconciling is done"); err != nil {

	}

	// sleep and check again
	return ctrl.Result{Requeue: true,
		RequeueAfter: time.Duration(instance.Spec.SyncSecInterval) * time.Second}, nil

}

// Finalizers will perform the required operations before delete the CR.
func (r *ProjectSetSyncReconciler) doFinalizerOperations(cr *projectv1alpha1.ProjectSetSync) {
	// TODO(user): Add the cleanup steps that the operator
	// needs to do before the CR can be deleted. Examples
	// of finalizers include performing backups and deleting
	// resources that are not owned by this CR, like a PVC.

	// Note: It is not recommended to use finalizers with the purpose of delete resources which are
	// created and managed in the reconciliation. These ones, such as the Deployment created on this reconcile,
	// are defined as depended of the custom resource. See that we use the method ctrl.SetControllerReference.
	// to set the ownerRef which means that the Deployment will be deleted by the Kubernetes API.
	// More info: https://kubernetes.io/docs/tasks/administer-cluster/use-cascading-deletion/

	//eventtype is the type of this event, and is either Normal or Warning.
	// The following implementation will raise an event

	// rm dir

	localDir := "/tmp/" + cr.Spec.EnvName

	log.Info("Remove repo dir")

	err := os.RemoveAll(localDir)

	if err != nil {
		log.Error(err, "Error removing repo dir")
	}

	//r.Recorder.Event(cr,
	//	"Warning",
	//	"Deleting",
	//	fmt.Sprintf("Custom Resource %s is being deleted", cr.Name))

}

type Config map[string]map[string]map[string]string

func (r *ProjectSetSyncReconciler) readConfig(ctx context.Context,
	req ctrl.Request,
	instance *projectv1alpha1.ProjectSetSync) (Config, error) {

	confPath := "/tmp/" + instance.Spec.EnvName + "/" + instance.Spec.ConfFile

	file, err := os.Open(confPath)
	if err != nil {
		log.Error(err, "Error reading repo conf file")
		return nil, err
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)

	var config Config

	// Decode the YAML data into the struct
	if err := decoder.Decode(&config); err != nil {
		log.Error(err, "Error decoding YAML data")
		return nil, err
	}

	log.Info("config", "data", config)

	return config, nil

}

// createOrUpdateCustomResource creates or updates the given Custom Resource
func (r *ProjectSetSyncReconciler) createOrUpdateProjectSetTemplate(ctx context.Context, req ctrl.Request, projectset *projectv1alpha1.ProjectSetTemplate) error {
	// Check if the Custom Resource already exists

	found := &projectv1alpha1.ProjectSetTemplate{}
	err := r.Get(ctx, types.NamespacedName{Name: projectset.Name}, found)

	//log.Info("GET error", "err", err)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Custom Resource not found, create it
			err = r.Create(ctx, projectset)
			if err != nil {
				log.Error(err, "Error create projectset")
				return err
			}
			return nil
		}

		//log.Error(err, "Error GET")
		// Error occurred while retrieving the Custom Resource
		return err
	}

	// Custom Resource found, update it
	found.Spec = projectset.Spec // Update with desired state
	err = r.Update(ctx, found)
	if err != nil {
		log.Error(err, "Error update projectset")
		return err
	}
	return nil
}

// createOrUpdateCustomResource creates or updates the given Custom Resource
func (r *ProjectSetSyncReconciler) createOrUpdateProjectSet(ctx context.Context, req ctrl.Request, projectset *projectv1alpha1.ProjectSet) error {
	// Check if the Custom Resource already exists

	projectset.Spec.LimitRange.Limits = getOrDefaultLimitRange(projectset)

	found := &projectv1alpha1.ProjectSet{}
	err := r.Get(ctx, types.NamespacedName{Name: projectset.Name}, found)

	//log.Info("GET error", "err", err)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Custom Resource not found, create it
			err = r.Create(ctx, projectset)
			if err != nil {
				log.Error(err, "Error create projectset")
				return err
			}
			return nil
		}

		//log.Error(err, "Error GET")
		// Error occurred while retrieving the Custom Resource
		return err
	}

	// Custom Resource found, update it
	found.Spec = projectset.Spec // Update with desired state
	err = r.Update(ctx, found)
	if err != nil {
		log.Error(err, "Error update projectset")
		return err
	}
	return nil
}

func (r *ProjectSetSyncReconciler) listClusterProjectSet() (map[string]bool, error) {

	listProjectSet := make(map[string]bool)

	// Retrieve network policies in the given namespace
	projectSetList := &projectv1alpha1.ProjectSetList{}
	err := r.List(context.TODO(), projectSetList, &client.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, ps := range projectSetList.Items {
		log.Info("project set", "name", ps.Name)
		listProjectSet[ps.Name] = true
	}

	return listProjectSet, nil

}

func (r *ProjectSetSyncReconciler) listClusterProjectSetTemplate() (map[string]bool, error) {

	listProjectSet := make(map[string]bool)

	// Retrieve network policies in the given namespace
	projectSetList := &projectv1alpha1.ProjectSetTemplateList{}
	err := r.List(context.TODO(), projectSetList, &client.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, ps := range projectSetList.Items {
		log.Info("project set template", "name", ps.Name)
		listProjectSet[ps.Name] = true
	}

	return listProjectSet, nil

}

func (r *ProjectSetSyncReconciler) applyProjectSetTemplate(ctx context.Context, req ctrl.Request,
	crdsPath []string) error {

	// check if needs to delete
	existingList, err := r.listClusterProjectSetTemplate()
	if err != nil {
		log.Error(err, "Failed to get listClusterProjectSetTemplate")
	}

	for _, file := range crdsPath {

		log.Info("Read yaml", "file", file)

		// Read the content of the YAML file
		yamlFile, err := os.ReadFile(file)
		if err != nil {
			log.Error(err, "Error reading YAML file")
		}

		obj := &projectv1alpha1.ProjectSetTemplate{}

		// Unmarshal the YAML content into the unstructured object
		decoder := apiyaml.NewYAMLOrJSONDecoder(bytes.NewReader(yamlFile), 4096)
		if err := decoder.Decode(obj); err != nil {
			log.Error(err, "Error decoding YAML content")
		}

		log.Info("Parse k8s obj", "obj", obj)

		// Apply the Custom Resource
		err = r.createOrUpdateProjectSetTemplate(ctx, req, obj)
		if err != nil {
			log.Error(err, "Error applying Custom Resource")
		}

		delete(existingList, obj.Name)

	}

	// iterate thru old and delete
	for k, _ := range existingList {

		log.Info("Delete old project set template", "name", k)

		if err := r.checkAndDeleteProjectSetTemplate(ctx, req, k); err != nil {

			log.Error(err, "Failed to delete old ProjectSet")
			return nil
		}
	}

	return nil

}

func (r *ProjectSetSyncReconciler) applyProjectSet(ctx context.Context, req ctrl.Request,
	crdsPath []string) error {

	// check if needs to delete
	existingList, err := r.listClusterProjectSet()
	if err != nil {
		log.Error(err, "Failed to get listClusterProjectSet")
	}

	for _, file := range crdsPath {

		log.Info("Read yaml", "file", file)

		// Read the content of the YAML file
		yamlFile, err := os.ReadFile(file)
		if err != nil {
			log.Error(err, "Error reading YAML file")
		}

		obj := &projectv1alpha1.ProjectSet{}

		// Unmarshal the YAML content into the unstructured object
		decoder := apiyaml.NewYAMLOrJSONDecoder(bytes.NewReader(yamlFile), 4096)
		if err := decoder.Decode(obj); err != nil {
			log.Error(err, "Error decoding YAML content")
		}

		log.Info("Parse k8s obj", "obj", obj)

		// Apply the Custom Resource
		err = r.createOrUpdateProjectSet(ctx, req, obj)
		if err != nil {
			log.Error(err, "Error applying Custom Resource")
		}

		delete(existingList, obj.Name)

	}

	// iterate thru old and delete
	for k, _ := range existingList {

		log.Info("Delete old project set", "name", k)

		if err := r.checkAndDeleteProjectSet(ctx, req, k); err != nil {

			log.Error(err, "Failed to delete old ProjectSet")
			return nil
		}
	}

	return nil

}

func (r *ProjectSetSyncReconciler) checkAndDeleteProjectSetTemplate(ctx context.Context,
	req ctrl.Request,
	name string) error {

	found := &projectv1alpha1.ProjectSetTemplate{}
	err := r.Get(ctx, types.NamespacedName{Name: name}, found)

	if err != nil && apierrors.IsNotFound(err) {
		return nil

	} else if err != nil {

		log.Error(err, "Failed to get ProjectSet")
		// Reconcile failed due to error - requeue
		return err
	}

	// delete policy because not found in CR
	err = r.Delete(ctx, found)

	if err != nil {
		log.Error(err, "Failed to delete ProjectSet")
		return err
	}

	return nil

}

func (r *ProjectSetSyncReconciler) checkAndDeleteProjectSet(ctx context.Context,
	req ctrl.Request,
	name string) error {

	found := &projectv1alpha1.ProjectSet{}
	err := r.Get(ctx, types.NamespacedName{Name: name}, found)

	if err != nil && apierrors.IsNotFound(err) {
		return nil

	} else if err != nil {

		log.Error(err, "Failed to get ProjectSet")
		// Reconcile failed due to error - requeue
		return err
	}

	// delete policy because not found in CR
	err = r.Delete(ctx, found)

	if err != nil {
		log.Error(err, "Failed to delete ProjectSet")
		return err
	}

	return nil

}

func (r *ProjectSetSyncReconciler) readCRD(instance *projectv1alpha1.ProjectSetSync,
	crdsPath string) ([]string, error) {

	crds := []string{}

	dirPath := "/tmp/" + instance.Spec.EnvName + "/" + crdsPath

	log.Info("check CRD in dir", "dir", dirPath)

	// if dir not exists - skip
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		log.Info("Dir not found", "name", dirPath)
		return crds, nil
	}

	dir, err := os.Open(dirPath)
	if err != nil {
		log.Error(err, "Error reading repo dir")
		return nil, err
	}
	defer dir.Close()

	files, err := dir.Readdir(0)
	if err != nil {
		log.Error(err, "Error reading files dir")
		return nil, err
	}

	// collect files
	for _, file := range files {

		if strings.HasSuffix(file.Name(), ".yaml") {
			log.Info("readCRD", "file", file.Name())

			crds = append(crds, dirPath+"/"+file.Name())
		}

	}

	return crds, nil

}

func (r *ProjectSetSyncReconciler) setStatus(ctx context.Context,
	req ctrl.Request,
	instance *projectv1alpha1.ProjectSetSync,
	statusType string,
	status metav1.ConditionStatus,
	reason, message string) error {

	// Refetch last state
	// if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
	// 	log.Error(err, "Failed to re-fetch instance")
	// 	return err
	// }
	//
	// // Set condition
	// meta.SetStatusCondition(&instance.Status.Conditions,
	// 	metav1.Condition{Type: statusType,
	// 		Status:  status,
	// 		Reason:  reason,
	// 		Message: message})
	//
	// // Update state
	// if err := r.Status().Update(ctx, instance); err != nil {
	// 	log.Error(err, "Failed to update instance status")
	// 	return err
	// }
	//
	// if err := r.Update(ctx, instance); err != nil {
	// 	log.Error(err, "Failed to update instance")
	// 	return err
	// }
	//
	// // Refetch last state
	// if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
	// 	log.Error(err, "Failed to re-fetch instance")
	// 	return err
	// }

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ProjectSetSyncReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&projectv1alpha1.ProjectSetSync{}).
		Complete(r)
}

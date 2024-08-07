package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

type ResourceEvent struct {
	Object runtime.Object
	Action string
}

func main() {
	// Set up Kubernetes client configuration
	kubeconfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	)
	config, err := kubeconfig.ClientConfig()
	if err != nil {
		log.Fatalf("Error creating kubeconfig: %s", err.Error())
	}

	// Create the Kubernetes client
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error creating Kubernetes client: %s", err.Error())
	}

	// Get the label key from the environment variable with a default value
	labelKey := getEnv("RESPONSIBILITY_LABEL_KEY", "namespace-crawler-responsibility")

	// Get the label key for master namespace from the environment variable with a default value
	masterLabelValue := getEnv("RESPONSIBILITY_LABEL_MASTER_VALUE", "master")
	slaveLabelValue := getEnv("RESPONSIBILITY_LABEL_SLAVE_VALUE", "slave")

	// Get the label key for master namespace from the environment variable with a default value
	masterNamespaceKey := getEnv("NAMESPACE_LIST_KEY", "namespace-crawler-responsible-for")

	// Get the label key for master namespace from the environment variable with a default value
	namespaceSeperator := getEnv("NAMESPACE_VALUE_SEPERATOR", "__")

	fmt.Println("------------------- running with values ------------------- ")
	fmt.Println("RESPONSIBILITY_LABEL_KEY -- ", labelKey)
	fmt.Println("RESPONSIBILITY_LABEL_MASTER_VALUE -- ", masterLabelValue)
	fmt.Println("RESPONSIBILITY_LABEL_SLAVE_VALUE -- ", slaveLabelValue)
	fmt.Println("NAMESPACE_LIST_KEY -- ", masterNamespaceKey)
	fmt.Println("NAMESPACE_VALUE_SEPERATOR -- ", namespaceSeperator)

	// Create a shared informer factory
	informerFactory := informers.NewSharedInformerFactory(clientset, time.Minute*10)

	// Get the Secret informer
	secretInformer := informerFactory.Core().V1().Secrets().Informer()

	// Get the ConfigMap informer
	configMapInformer := informerFactory.Core().V1().ConfigMaps().Informer()

	// Get the Deployment informer
	deploymentInformer := informerFactory.Apps().V1().Deployments().Informer()

	// Channel to publish resource events
	resourceEvents := make(chan ResourceEvent, 100)

	// Set up event handlers for secrets
	secretInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			secret := obj.(*v1.Secret)
			if isMaster(secret.Labels, labelKey, masterLabelValue) {
				resourceEvents <- ResourceEvent{Object: secret, Action: "created"}
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			secret := newObj.(*v1.Secret)
			if isMaster(secret.Labels, labelKey, masterLabelValue) {
				resourceEvents <- ResourceEvent{Object: secret, Action: "updated"}
			}
		},
		DeleteFunc: func(obj interface{}) {
			secret := obj.(*v1.Secret)
			if isMaster(secret.Labels, labelKey, masterLabelValue) {
				resourceEvents <- ResourceEvent{Object: secret, Action: "deleted"}
			}
		},
	})

	// Set up event handlers for config maps
	configMapInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			configMap := obj.(*v1.ConfigMap)
			if isMaster(configMap.Labels, labelKey, masterLabelValue) {
				resourceEvents <- ResourceEvent{Object: configMap, Action: "created"}
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			configMap := newObj.(*v1.ConfigMap)
			if isMaster(configMap.Labels, labelKey, masterLabelValue) {
				resourceEvents <- ResourceEvent{Object: configMap, Action: "updated"}
			}
		},
		DeleteFunc: func(obj interface{}) {
			configMap := obj.(*v1.ConfigMap)
			if isMaster(configMap.Labels, labelKey, masterLabelValue) {
				resourceEvents <- ResourceEvent{Object: configMap, Action: "deleted"}
			}
		},
	})

	// Set up event handlers for deployments
	deploymentInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			deployment := obj.(*appsv1.Deployment)
			if isMaster(deployment.Labels, labelKey, masterLabelValue) {
				resourceEvents <- ResourceEvent{Object: deployment, Action: "created"}
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			deployment := newObj.(*appsv1.Deployment)
			if isMaster(deployment.Labels, labelKey, masterLabelValue) {
				resourceEvents <- ResourceEvent{Object: deployment, Action: "updated"}
			}
		},
		DeleteFunc: func(obj interface{}) {
			deployment := obj.(*appsv1.Deployment)
			if isMaster(deployment.Labels, labelKey, masterLabelValue) {
				resourceEvents <- ResourceEvent{Object: deployment, Action: "deleted"}
			}
		},
	})

	// Goroutine to handle resource events
	go func() {
		for event := range resourceEvents {
			switch resource := event.Object.(type) {
			case *v1.Secret:
				fmt.Printf("Secret %s in namespace %s was %s\n", resource.Name, resource.Namespace, event.Action)
				handleSecretEvent(clientset, resource, event.Action, labelKey, slaveLabelValue, masterNamespaceKey, namespaceSeperator)
			case *v1.ConfigMap:
				fmt.Printf("ConfigMap %s in namespace %s was %s\n", resource.Name, resource.Namespace, event.Action)
				handleConfigMapEvent(clientset, resource, event.Action, labelKey, slaveLabelValue, masterNamespaceKey, namespaceSeperator)
			case *appsv1.Deployment:
				fmt.Printf("Deployment %s in namespace %s was %s\n", resource.Name, resource.Namespace, event.Action)
				handleDeploymentEvent(clientset, resource, event.Action, labelKey, slaveLabelValue, masterNamespaceKey, namespaceSeperator)
			}
		}
	}()

	// Start the informer
	stopCh := make(chan struct{})
	defer close(stopCh)

	go informerFactory.Start(stopCh)

	// Wait for the caches to sync
	if !cache.WaitForCacheSync(stopCh, secretInformer.HasSynced, configMapInformer.HasSynced, deploymentInformer.HasSynced) {
		log.Fatalf("Error syncing cache")
	}

	// Wait for termination signals
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)
	<-signalCh

	close(resourceEvents)
}

// Check if the resource has the label "namespace-crawler-responsibility: master"
func isMaster(labels map[string]string, labelKey string, masterLabelValue string) bool {
	value, exists := labels[labelKey]
	return exists && value == masterLabelValue
}

// Handle secret events and create/update secrets in target namespaces
func handleSecretEvent(clientset *kubernetes.Clientset, secret *v1.Secret, event string, labelKey string, slaveLabelValue string, masterNamespaceKey string, namespaceSeperator string) {
	targetNamespaces, exists := secret.Labels[masterNamespaceKey]
	if !exists {
		fmt.Printf("Secret %s in namespace %s does not contain %s label\n", secret.Name, secret.Namespace, masterNamespaceKey)
		return
	}

	namespaces := strings.Split(targetNamespaces, namespaceSeperator)
	for _, ns := range namespaces {
		newSecret := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secret.Name,
				Namespace: ns,
				Labels: map[string]string{
					labelKey: slaveLabelValue,
				},
			},
			Data: secret.Data,
			Type: secret.Type,
		}
		if event == "deleted" {
			err := clientset.CoreV1().Secrets(ns).Delete(context.TODO(), secret.Name, metav1.DeleteOptions{})
			if err != nil {
				log.Printf("Error deleting secret %s in namespace %s: %s", secret.Name, ns, err.Error())
			} else {
				fmt.Printf("Secret %s deleted in namespace %s\n", secret.Name, ns)
			}
		} else {
			// Create or update the secret in the target namespace
			_, err := clientset.CoreV1().Secrets(ns).Get(context.TODO(), secret.Name, metav1.GetOptions{})
			if err != nil {
				// Secret does not exist, create it
				_, err = clientset.CoreV1().Secrets(ns).Create(context.TODO(), newSecret, metav1.CreateOptions{})
				if err != nil {
					log.Printf("Error creating secret %s in namespace %s: %s", secret.Name, ns, err.Error())
				} else {
					fmt.Printf("Secret %s created in namespace %s\n", secret.Name, ns)
				}
			} else {
				// Secret exists, update it
				_, err = clientset.CoreV1().Secrets(ns).Update(context.TODO(), newSecret, metav1.UpdateOptions{})
				if err != nil {
					log.Printf("Error updating secret %s in namespace %s: %s", secret.Name, ns, err.Error())
				} else {
					fmt.Printf("Secret %s updated in namespace %s\n", secret.Name, ns)
				}
			}
		}
	}
}

// Handle configmap events and create/update configmaps in target namespaces
func handleConfigMapEvent(clientset *kubernetes.Clientset, configMap *v1.ConfigMap, event string, labelKey string, slaveLabelValue string, masterNamespaceKey string, namespaceSeperator string) {
	targetNamespaces, exists := configMap.Labels[masterNamespaceKey]
	if !exists {
		fmt.Printf("ConfigMap %s in namespace %s does not contain %s label\n", configMap.Name, configMap.Namespace, masterNamespaceKey)
		return
	}

	namespaces := strings.Split(targetNamespaces, namespaceSeperator)
	for _, ns := range namespaces {
		newConfigMap := &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      configMap.Name,
				Namespace: ns,
				Labels: map[string]string{
					labelKey: slaveLabelValue,
				},
			},
			Data: configMap.Data,
		}
		if event == "deleted" {
			err := clientset.CoreV1().ConfigMaps(ns).Delete(context.TODO(), configMap.Name, metav1.DeleteOptions{})
			if err != nil {
				log.Printf("Error deleting configMap %s in namespace %s: %s", configMap.Name, ns, err.Error())
			} else {
				fmt.Printf("ConfigMap %s deleted in namespace %s\n", configMap.Name, ns)
			}
		} else {

			// Create or update the configmap in the target namespace
			_, err := clientset.CoreV1().ConfigMaps(ns).Get(context.TODO(), configMap.Name, metav1.GetOptions{})
			if err != nil {
				// ConfigMap does not exist, create it
				_, err = clientset.CoreV1().ConfigMaps(ns).Create(context.TODO(), newConfigMap, metav1.CreateOptions{})
				if err != nil {
					log.Printf("Error creating configmap %s in namespace %s: %s", configMap.Name, ns, err.Error())
				} else {
					fmt.Printf("ConfigMap %s created in namespace %s\n", configMap.Name, ns)
				}
			} else {
				// ConfigMap exists, update it
				_, err = clientset.CoreV1().ConfigMaps(ns).Update(context.TODO(), newConfigMap, metav1.UpdateOptions{})
				if err != nil {
					log.Printf("Error updating configmap %s in namespace %s: %s", configMap.Name, ns, err.Error())
				} else {
					fmt.Printf("ConfigMap %s updated in namespace %s\n", configMap.Name, ns)
				}
			}
		}
	}
}

// Handle deployment events and create/update deployments in target namespaces
func handleDeploymentEvent(clientset *kubernetes.Clientset, deployment *appsv1.Deployment, event string, labelKey string, slaveLabelValue string, masterNamespaceKey string, namespaceSeperator string) {
	targetNamespaces, exists := deployment.Labels[masterNamespaceKey]
	if !exists {
		fmt.Printf("Deployment %s in namespace %s does not contain %s label\n", deployment.Name, deployment.Namespace, masterNamespaceKey)
		return
	}

	namespaces := strings.Split(targetNamespaces, namespaceSeperator)
	for _, ns := range namespaces {
		newDeployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      deployment.Name,
				Namespace: ns,
				Labels: map[string]string{
					labelKey: slaveLabelValue,
				},
			},
			Spec: deployment.Spec,
		}
		if event == "deleted" {
			err := clientset.AppsV1().Deployments(ns).Delete(context.TODO(), deployment.Name, metav1.DeleteOptions{})
			if err != nil {
				log.Printf("Error deleting deployment %s in namespace %s: %s", deployment.Name, ns, err.Error())
			} else {
				fmt.Printf("Deployment %s deleted in namespace %s\n", deployment.Name, ns)
			}
		} else {
			// Create or update the deployment in the target namespace
			_, err := clientset.AppsV1().Deployments(ns).Get(context.TODO(), deployment.Name, metav1.GetOptions{})
			if err != nil {
				// Deployment does not exist, create it
				_, err = clientset.AppsV1().Deployments(ns).Create(context.TODO(), newDeployment, metav1.CreateOptions{})
				if err != nil {
					log.Printf("Error creating deployment %s in namespace %s: %s", deployment.Name, ns, err.Error())
				} else {
					fmt.Printf("Deployment %s created in namespace %s\n", deployment.Name, ns)
				}
			} else {
				// Deployment exists, update it
				_, err = clientset.AppsV1().Deployments(ns).Update(context.TODO(), newDeployment, metav1.UpdateOptions{})
				if err != nil {
					log.Printf("Error updating deployment %s in namespace %s: %s", deployment.Name, ns, err.Error())
				} else {
					fmt.Printf("Deployment %s updated in namespace %s\n", deployment.Name, ns)
				}
			}
		}
	}
}

// Get the environment variable value or return a default
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

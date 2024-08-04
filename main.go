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

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

type SecretEvent struct {
	Secret *v1.Secret
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

	fmt.Println("------------------- runnning with values ------------------- ")
	fmt.Println("RESPONSIBILITY_LABEL_KEY -- ", labelKey)
	fmt.Println("RESPONSIBILITY_LABEL_MASTER_VALUE -- ", masterLabelValue)
	fmt.Println("RESPONSIBILITY_LABEL_SLAVE_VALUE -- ", slaveLabelValue)
	fmt.Println("NAMESPACE_LIST_KEY -- ", masterNamespaceKey)
	fmt.Println("NAMESPACE_VALUE_SEPERATOR -- ", namespaceSeperator)

	// Create a shared informer factory
	informerFactory := informers.NewSharedInformerFactory(clientset, time.Minute*10)

	// Get the Secret informer
	secretInformer := informerFactory.Core().V1().Secrets().Informer()

	// Channel to publish secret events
	secretEvents := make(chan SecretEvent, 100)

	// Set up event handlers
	secretInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			secret := obj.(*v1.Secret)
			if isMaster(secret, labelKey, masterLabelValue) {
				secretEvents <- SecretEvent{Secret: secret, Action: "created"}
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			secret := newObj.(*v1.Secret)
			if isMaster(secret, labelKey, masterLabelValue) {
				secretEvents <- SecretEvent{Secret: secret, Action: "updated"}
			}
		},
		DeleteFunc: func(obj interface{}) {
			secret := obj.(*v1.Secret)
			if isMaster(secret, labelKey, masterLabelValue) {
				secretEvents <- SecretEvent{Secret: secret, Action: "deleted"}
			}
		},
	})

	// Goroutine to handle secret events
	go func() {
		for event := range secretEvents {
			fmt.Printf("Secret %s in namespace %s was %s\n", event.Secret.Name, event.Secret.Namespace, event.Action)
			handleSecretEvent(clientset, event.Secret, labelKey, slaveLabelValue, masterNamespaceKey, namespaceSeperator)
		}
	}()

	// Start the informer
	stopCh := make(chan struct{})
	defer close(stopCh)

	go informerFactory.Start(stopCh)

	// Wait for the caches to sync
	if !cache.WaitForCacheSync(stopCh, secretInformer.HasSynced) {
		log.Fatalf("Error syncing cache")
	}

	// Wait for termination signals
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)
	<-signalCh

	close(secretEvents)
}

// Check if the secret has the label "namespace-crawler-responsibility: master"
func isMaster(secret *v1.Secret, labelKey string, masterLabelValue string) bool {
	value, exists := secret.Labels[labelKey]
	return exists && value == masterLabelValue
}

// Handle secret events and create/update secrets in target namespaces
func handleSecretEvent(clientset *kubernetes.Clientset, secret *v1.Secret, labelKey string, slaveLabelValue string, masterNamespaceKey string, namespaceSeperator string) {
	targetNamespaces, exists := secret.Labels[masterNamespaceKey]
	if !exists {
		fmt.Printf("Secret %s in namespace %s  don't contains %s label", secret.Name, secret.Namespace, masterNamespaceKey)
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

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

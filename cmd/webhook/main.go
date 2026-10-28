package main

import (
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/AobaIwaki123/gomemlimit-webhook/pkg/cert"
	"github.com/AobaIwaki123/gomemlimit-webhook/pkg/mutator"
)

func main() {
	var (
		port         int
		certFile     string
		keyFile      string
		autoCert     bool
		serviceName  string
		namespace    string
		printCABundle bool
	)

	flag.IntVar(&port, "port", 8443, "Port to listen on for webhook requests")
	flag.StringVar(&certFile, "tls-cert-file", "", "Path to TLS certificate file")
	flag.StringVar(&keyFile, "tls-key-file", "", "Path to TLS private key file")
	flag.BoolVar(&autoCert, "auto-cert", true, "Generate self-signed certificate automatically in memory")
	flag.StringVar(&serviceName, "service-name", "gomemlimit-webhook", "Kubernetes Service name for auto-cert SANs")
	flag.StringVar(&namespace, "namespace", "default", "Kubernetes Namespace for auto-cert SANs")
	flag.BoolVar(&printCABundle, "print-ca-bundle", false, "Print base64-encoded CA bundle and exit (useful for bootstrapping MutatingWebhookConfiguration)")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	m := mutator.NewMutator()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.HandleFunc("POST /mutate", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			logger.Error("failed to read request body", "error", err)
			http.Error(w, "could not read request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		var review admissionv1.AdmissionReview
		if err := json.Unmarshal(body, &review); err != nil {
			logger.Error("failed to unmarshal AdmissionReview", "error", err)
			http.Error(w, "could not parse admission review", http.StatusBadRequest)
			return
		}

		response := m.Mutate(&review)
		reviewResponse := admissionv1.AdmissionReview{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "admission.k8s.io/v1",
				Kind:       "AdmissionReview",
			},
			Response: response,
		}

		respBytes, err := json.Marshal(reviewResponse)
		if err != nil {
			logger.Error("failed to marshal AdmissionReview response", "error", err)
			http.Error(w, "could not encode response", http.StatusInternalServerError)
			return
		}

		logger.Info("processed admission review",
			"uid", response.UID,
			"allowed", response.Allowed,
			"patch_applied", len(response.Patch) > 0,
		)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(respBytes)
	})

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	var tlsConfig *tls.Config

	if autoCert {
		logger.Info("generating in-memory self-signed TLS certificates",
			"service", serviceName,
			"namespace", namespace,
		)
		caPair, serverPair, err := cert.GenerateSelfSignedCert(serviceName, namespace)
		if err != nil {
			logger.Error("failed to generate self-signed cert", "error", err)
			os.Exit(1)
		}

		if printCABundle {
			// Print for MutatingWebhookConfiguration caBundle
			fmt.Printf("CA_BUNDLE_BASE64=%s\n", cert.EncodeCABundle(caPair.CertPEM))
			return
		}

		tlsCert, err := tls.X509KeyPair(serverPair.CertPEM, serverPair.KeyPEM)
		if err != nil {
			logger.Error("failed to create TLS key pair from memory", "error", err)
			os.Exit(1)
		}

		tlsConfig = &tls.Config{
			Certificates: []tls.Certificate{tlsCert},
		}
		server.TLSConfig = tlsConfig
	}

	go func() {
		logger.Info("starting webhook HTTPS server", "port", port)
		var err error
		if autoCert {
			err = server.ListenAndServeTLS("", "")
		} else {
			err = server.ListenAndServeTLS(certFile, keyFile)
		}
		if err != nil && err != http.ErrServerClosed {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, syscall.SIGINT, syscall.SIGTERM)
	<-stopCh

	logger.Info("shutting down webhook server gracefully...")
	_ = server.Close()
}

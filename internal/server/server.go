package server

import (
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

func Server(listenAddress string) *http.Server {
	router := http.NewServeMux()

	router.Handle("/metrics", promhttp.Handler())

	logger := logrus.New()
	w := logger.Writer()
	defer w.Close()

	server := &http.Server{
		Addr:     listenAddress,
		Handler:  router,
		ErrorLog: log.New(w, "", 0),
	}

	return server
}

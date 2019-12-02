// Copyright 2017 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package prometheus

import (
	"fmt"
	"github.com/palletone/go-palletone/common"
	"github.com/palletone/go-palletone/common/log"
	"github.com/palletone/go-palletone/common/p2p"
	"github.com/palletone/go-palletone/common/rpc"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"os"
)

// Prometheus contains the dashboard internals.
type Prometheus struct {
	config Config
}

// New creates a new dashboard instance with the given configuration.
func New(config Config) (*Prometheus, error) {
	return &Prometheus{config: config}, nil
}

// Protocols is a meaningless implementation of node.Service.
func (db *Prometheus) Protocols() []p2p.Protocol { return nil }

func (db *Prometheus) GenesisHash() common.Hash      { return common.Hash{} }
func (db *Prometheus) CorsProtocols() []p2p.Protocol { return nil }

// APIs is a meaningless implementation of node.Service.
func (db *Prometheus) APIs() []rpc.API { return nil }

// Start implements node.Service, starting the data collection thread and the listening server of the dashboard.
func (db *Prometheus) Start(server *p2p.Server, corss *p2p.Server) error {
	log.Info("Starting Prometheus")
	go func() {
		http.Handle("/", promhttp.Handler())
		///api/v1/query
		http.Handle("/api/v1/query", db.QueryHandler())

		if err := http.ListenAndServe(fmt.Sprintf("%s:%d", db.config.Host, db.config.Port), nil); err != nil {
			log.Error("Failed to starting prometheus", "err", err)
			os.Exit(1)
		}
	}()

	return nil
}
func (db *Prometheus) QueryHandler() http.Handler {
	return promhttp.InstrumentMetricHandler(
		prometheus.DefaultRegisterer, db.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{}),
	)
}

func (db *Prometheus) HandlerFor(reg prometheus.Gatherer, opts promhttp.HandlerOpts) http.Handler {
	h := http.HandlerFunc(func(rsp http.ResponseWriter, req *http.Request) {
		log.Debug("======HandlerFor======")
		rsp.WriteHeader(200)
	})
	return h
}

// Stop implements node.Service, stopping the data collection thread and the connection listener of the dashboard.
func (db *Prometheus) Stop() error {
	// Close the connection listener.

	log.Info("Prometheus stopped")

	return nil
}

package chartstreams

import (
	"bytes"
	"fmt"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
	"helm.sh/helm/v3/pkg/releaseutil"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/contrib/ginrus"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/chart/loader"

	"github.com/otaviof/chart-streams/pkg/chartstreams/config"
	"github.com/otaviof/chart-streams/pkg/chartstreams/provider"
)

// ChartStreamServer represents the chartstreams server offering its API. The server puts together
// the routes, and bootstrap steps in order to respond as a valid Helm repository.
type ChartStreamServer struct {
	config        *config.Config
	chartProvider provider.ChartProvider
}

// Start executes the boostrap steps in order to start listening on configured address. It can return
// errors from "listen" method.
func (s *ChartStreamServer) Start() error {
	if err := s.chartProvider.Initialize(); err != nil {
		return err
	}

	return s.listen()
}

// IndexHandler endpoint handler to render a index.yaml file.
func (s *ChartStreamServer) IndexHandler(c *gin.Context) {
	index, err := s.chartProvider.GetIndexFile()
	if err != nil {
		_ = c.AbortWithError(500, err)
	}

	c.YAML(200, index)
}

// TemplatePayload encodes the contract the client uses to render a chart template.
type TemplatePayload struct {
	Name      string           `json:"name"`
	Values    chartutil.Values `json:"values,omitempty"`
	Namespace string           `json:"namespace"`
	Revision  int              `json:"revision"`
}

// renderedManifests adds specific functionality to rendered chart outcomes.
type renderedManifests map[string]string

func (r renderedManifests) AsBytes() []byte {
	_, manifests, err := releaseutil.SortManifests(r, chartutil.VersionSet(releaseutil.InstallOrder),
		releaseutil.InstallOrder)
	if err != nil {
		fmt.Printf(err.Error())
	}

	b := bytes.NewBuffer([]byte{})
	for _, m := range manifests {
		content := "\n---\n" + m.Content
		_, err := b.Write([]byte(content))
		if err != nil {
			panic(err)
		}
	}

	return b.Bytes()
}

// TemplateHandler endpoint handler used to render a specific chart version.
func (s *ChartStreamServer) TemplateHandler(c *gin.Context) {
	name := c.Param("name")
	version := c.Param("version")
	version = strings.TrimPrefix(version, "/")

	p, err := s.chartProvider.GetChart(name, version)
	if err != nil {
		_ = c.AbortWithError(500, err)
	}

	chartBytes := p.BytesBuffer.Bytes()
	chart, err := loader.LoadArchive(bytes.NewReader(chartBytes))
	if err != nil {
		_ = c.AbortWithError(500, err)
	}

	payload := &TemplatePayload{}
	err = c.Bind(payload)

	options := chartutil.ReleaseOptions{
		Name:      payload.Name,
		Namespace: payload.Namespace,
		IsInstall: true,
		IsUpgrade: false,
		Revision:  payload.Revision,
	}

	vals, err := chartutil.ToRenderValues(chart, payload.Values, options, nil)
	if err != nil {
		_ = c.AbortWithError(500, err)
	}

	if err := chartutil.ValidateAgainstSchema(chart, vals); err != nil {
		_ = c.AbortWithError(500, err)
	}

	r, err := engine.Render(chart, vals)
	if err != nil {
		_ = c.AbortWithError(500, err)
	}

	asBytes := renderedManifests(r).AsBytes()

	c.Data(200, gin.MIMEYAML, asBytes)
}

// DirectLinkHandler endpoint handler to directly load a chart tarball payload.
func (s *ChartStreamServer) DirectLinkHandler(c *gin.Context) {
	name := c.Param("name")
	version := c.Param("version")
	version = strings.TrimPrefix(version, "/")

	p, err := s.chartProvider.GetChart(name, version)
	if err != nil {
		_ = c.AbortWithError(500, err)
	}

	c.Data(http.StatusOK, "application/gzip", p.Bytes())
}

// listen on configured address, after adding the route handlers to the framework. It can return
// errors coming from Gin.
func (s *ChartStreamServer) listen() error {
	g := gin.New()

	g.Use(ginrus.Ginrus(log.StandardLogger(), time.RFC3339, true))

	g.GET("/index.yaml", s.IndexHandler)
	g.GET("/chart/:name/*version", s.DirectLinkHandler)
	g.POST("/template/:name/*version", s.TemplateHandler)

	return g.Run(s.config.ListenAddr)
}

// NewServer instantiate a new server instance.
func NewServer(config *config.Config) *ChartStreamServer {
	p := provider.NewGitChartProvider(config)
	return &ChartStreamServer{
		config:        config,
		chartProvider: p,
	}
}

package handlers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/gofrs/uuid"
	"github.com/gorilla/mux"

	"github.com/meshery/meshery/server/helpers"
	"github.com/meshery/meshery/server/helpers/utils"
	"github.com/meshery/meshery/server/models"
	"github.com/meshery/meshery/server/models/pattern/core"

	// "github.com/meshery/meshkit/errors"
	// "github.com/meshery/meshkit/errors"
	"github.com/meshery/meshkit/models/events"

	meshkitOci "github.com/meshery/meshkit/models/oci"
	"github.com/meshery/meshkit/models/registration"
	meshkitRegistryUtils "github.com/meshery/meshkit/registry"
	meshkitutils "github.com/meshery/meshkit/utils"

	_models "github.com/meshery/meshkit/models/meshmodel/core/v1beta1"
	"github.com/meshery/schemas/models/v1alpha3/relationship"
	schemav1beta1 "github.com/meshery/schemas/models/v1beta1"
	"github.com/meshery/schemas/models/v1beta1/component"
	"github.com/meshery/schemas/models/v1beta1/connection"
	_model "github.com/meshery/schemas/models/v1beta1/model"

	"github.com/meshery/meshkit/models/meshmodel/entity"
	"github.com/meshery/meshkit/models/meshmodel/registry"

	regv1beta1 "github.com/meshery/meshkit/models/meshmodel/registry/v1beta1"
)

/**Meshmodel endpoints **/
const DefaultPageSizeForMeshModelComponents = 25

// swagger:route GET /api/meshmodels/categories/{category}/models GetMeshmodelModelsByCategories idGetMeshmodelModelsByCategories
//
// Handle GET request for getting all meshmodel models for a given category. The component type/model name should be lowercase like "kubernetes", "istio"
//
// ```?version={version}``` If version is unspecified then all models are returned
//
// ```?order={field}``` orders on the passed field
//
// ```?search={modelname}``` If search is non empty then a greedy search is performed
//
// ```?sort={[asc/desc]}``` Default behavior is asc
//
// ```?page={page-number}``` Default page number is 1
//
// ```?pagesize={pagesize}``` Default pagesize is 25. To return all results: ```pagesize=all```
// ```?annotations={["true"/"false"/]}``` When this query parameter is "true", only models with the "isAnnotation" property set to true are returned. When  this query parameter is "false", all models except those considered to be annotation models are returned. Any other value of the query parameter results in both annoations as well as non-annotation models being returned.
// responses:
// ```?annotations={["true"/"false"/]}``` If "true" models having "isAnnotation" property as true are "only" returned, If false all models except "annotations" are returned. Any other value of the query parameter results in both annoations as well as non-annotation models being returned.
//
//	200: []meshmodelModelsDuplicateResponseWrapper
func (h *Handler) GetMeshmodelModelsByCategories(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Add("Content-Type", "application/json")
	enc := json.NewEncoder(rw)
	cat := mux.Vars(r)["category"]
	queryParams := r.URL.Query()
	page, offset, limit, search, order, sort, _ := getPaginationParams(r)
	returnAnnotationComp := queryParams.Get("annotations")

	filter := &regv1beta1.ModelFilter{
		Category:    cat,
		Version:     queryParams.Get("version"),
		Limit:       limit,
		Offset:      offset,
		OrderOn:     order,
		Sort:        sort,
		Annotations: returnAnnotationComp,
	}
	if search != "" {
		filter.Greedy = true
		filter.DisplayName = search
	}
	entities, count, _, _ := h.registryManager.GetEntities(filter)
	var modelDefs []_model.ModelDefinition
	for _, model := range entities {
		model, ok := model.(*_model.ModelDefinition)
		if ok {
			modelDefs = append(modelDefs, *model)
		}
	}

	var pgSize int64
	if limit == 0 {
		pgSize = count
	} else {
		pgSize = int64(limit)
	}

	res := models.MeshmodelsDuplicateAPIResponse{
		Page:     page,
		PageSize: int(pgSize),
		Count:    count,
		Models:   models.FindDuplicateModels(modelDefs),
	}

	if err := enc.Encode(res); err != nil {
		h.log.Error(ErrGetMeshModels(err)) //TODO: Add appropriate meshkit error
		http.Error(rw, ErrGetMeshModels(err).Error(), http.StatusInternalServerError)
	}
}

// swagger:route GET /api/meshmodels/categories/{category}/models/{model} GetMeshmodelModelsByCategoriesByModel idGetMeshmodelModelsByCategoriesByModel
//
// Handle GET request for getting all meshmodel models for a given category. The component type/model name should be lowercase like "kubernetes", "istio"
//
// ```?version={version}``` If version is unspecified then all models are returned
//
// ```?order={field}``` orders on the passed field
//
// ```?search={[true/false]}``` If search is true then a greedy search is performed
//
// ```?sort={[asc/desc]}``` Default behavior is asc
//
// ```?page={page-number}``` Default page number is 1
//
// ```?pagesize={pagesize}``` Default pagesize is 25. To return all results: ```pagesize=all```
// responses:
// ```?annotations={["true"/"false"/]}``` If "true" models having "isAnnotation" property as true are "only" returned, If false all models except "annotations" are returned. Any other value of the query parameter results in both annoations as well as non-annotation models being returned.
//
//	200: []meshmodelModelsDuplicateResponseWrapper
func (h *Handler) GetMeshmodelModelsByCategoriesByModel(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Add("Content-Type", "application/json")
	enc := json.NewEncoder(rw)
	cat := mux.Vars(r)["category"]
	model := mux.Vars(r)["model"]
	queryParams := r.URL.Query()
	page, offset, limit, search, order, sort, _ := getPaginationParams(r)
	var greedy bool
	if search == "true" {
		greedy = true
	}
	returnAnnotationComp := queryParams.Get("annotations")

	entities, count, _, _ := h.registryManager.GetEntities(&regv1beta1.ModelFilter{
		Category:    cat,
		Name:        model,
		Version:     queryParams.Get("version"),
		Limit:       limit,
		Offset:      offset,
		Greedy:      greedy,
		OrderOn:     order,
		Sort:        sort,
		Annotations: returnAnnotationComp,
	})

	var modelDefs []_model.ModelDefinition
	for _, model := range entities {
		model, ok := model.(*_model.ModelDefinition)
		if ok {
			modelDefs = append(modelDefs, *model)
		}
	}

	var pgSize int64
	if limit == 0 {
		pgSize = count
	} else {
		pgSize = int64(limit)
	}

	res := models.MeshmodelsDuplicateAPIResponse{
		Page:     page,
		PageSize: int(pgSize),
		Count:    count,
		Models:   models.FindDuplicateModels(modelDefs),
	}

	if err := enc.Encode(res); err != nil {
		h.log.Error(ErrGetMeshModels(err)) //TODO: Add appropriate meshkit error
		http.Error(rw, ErrGetMeshModels(err).Error(), http.StatusInternalServerError)
	}
}

// swagger:route GET /api/meshmodels/models GetMeshmodelModels idGetMeshmodelModels
// Handle GET request for getting all meshmodel models
//
// # Returns a list of registered models across all categories
//
// ```?version={version}``` If version is unspecified then all models are returned
//
// ```?order={field}``` orders on the passed field
//
// ```?search={modelname}``` If search is non empty then a greedy search is performed
//
// ```?sort={[asc/desc]}``` Default behavior is asc
//
// ```?page={page-number}``` Default page number is 1
//
// ```?pagesize={pagesize}``` Default pagesize is 25. To return all results: ```pagesize=all```
// responses:
// ```?annotations={["true"/"false"/]}``` If "true" models having "isAnnotation" property as true are "only" returned, If false all models except "annotations" are returned. Any other value of the query parameter results in both annoations as well as non-annotation models being returned.
//
//	200: meshmodelModelsDuplicateResponseWrapper
func (h *Handler) GetMeshmodelModels(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Add("Content-Type", "application/json")
	enc := json.NewEncoder(rw)
	queryParams := r.URL.Query()
	page, offset, limit, search, order, sort, _ := getPaginationParams(r)
	v := queryParams.Get("version")
	returnAnnotationComp := queryParams.Get("annotations")

	filter := &regv1beta1.ModelFilter{
		Id:          queryParams.Get("id"),
		Registrant:  queryParams.Get("registrant"),
		Version:     v,
		Limit:       limit,
		Offset:      offset,
		OrderOn:     order,
		Sort:        sort,
		Annotations: returnAnnotationComp,

		Components:    queryParams.Get("components") == "true",
		Relationships: queryParams.Get("relationships") == "true",
		Status:        queryParams.Get("status"),
		Trim:          queryParams.Get("trim") == "true",
	}
	if search != "" {
		filter.DisplayName = search
		filter.Name = search
		filter.Greedy = true
	}

	entities, count, _, _ := h.registryManager.GetEntities(filter)
	var modelDefs []_model.ModelDefinition
	for _, model := range entities {
		model, ok := model.(*_model.ModelDefinition)
		if ok {
			modelDefs = append(modelDefs, *model)
		}
	}
	var pgSize int64
	if limit == 0 {
		pgSize = count
	} else {
		pgSize = int64(limit)
	}

	res := models.MeshmodelsDuplicateAPIResponse{
		Page:     page,
		PageSize: int(pgSize),
		Count:    count,
		Models:   models.FindDuplicateModels(modelDefs),
	}

	if err := enc.Encode(res); err != nil {
		h.log.Error(ErrGetMeshModels(err)) //TODO: Add appropriate meshkit error
		http.Error(rw, ErrGetMeshModels(err).Error(), http.StatusInternalServerError)
	}
}

// swagger:route GET /api/meshmodels/models/{model} GetMeshmodelModelsByName idGetMeshmodelModelsByName
// Handle GET request for getting all meshmodel models. The component type/model name should be lowercase like "kubernetes", "istio"
//
// # Returns a list of registered models across all categories
//
// ```?version={version}``` If version is unspecified then all models are returned
//
// ```?order={field}``` orders on the passed field
//
// ```?search={[true/false]}``` If search is true then a greedy search is performed
//
// ```?sort={[asc/desc]}``` Default behavior is asc
//
// ```?page={page-number}``` Default page number is 1
//
// ```?pagesize={pagesize}``` Default pagesize is 25. To return all results: ```pagesize=all```
// responses:
// ```?annotations={["true"/"false"/]}``` If "true" models having "isAnnotation" property as true are "only" returned, If false all models except "annotations" are returned. Any other value of the query parameter results in both annoations as well as non-annotation models being returned.
//
//	200: []meshmodelModelsDuplicateResponseWrapper
func (h *Handler) GetMeshmodelModelsByName(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Add("Content-Type", "application/json")
	enc := json.NewEncoder(rw)
	name := mux.Vars(r)["model"]
	queryParams := r.URL.Query()
	page, offset, limit, search, order, sort, _ := getPaginationParams(r)
	var greedy bool
	if search == "true" {
		greedy = true
	}
	v := queryParams.Get("version")
	returnAnnotationComp := queryParams.Get("annotations")
	entities, count, _, _ := h.registryManager.GetEntities(&regv1beta1.ModelFilter{
		Name:        name,
		Version:     v,
		Limit:       limit,
		Offset:      offset,
		Greedy:      greedy,
		OrderOn:     order,
		Sort:        sort,
		Annotations: returnAnnotationComp,

		Components:    queryParams.Get("components") == "true",
		Relationships: queryParams.Get("relationships") == "true",
	})

	var modelDefs []_model.ModelDefinition
	for _, model := range entities {
		model, ok := model.(*_model.ModelDefinition)
		if ok {
			modelDefs = append(modelDefs, *model)
		}
	}

	var pgSize int64
	if limit == 0 {
		pgSize = count
	} else {
		pgSize = int64(limit)
	}

	res := models.MeshmodelsDuplicateAPIResponse{
		Page:     page,
		PageSize: int(pgSize),
		Count:    count,
		Models:   models.FindDuplicateModels(modelDefs),
	}

	if err := enc.Encode(res); err != nil {
		h.log.Error(ErrGetMeshModels(err)) //TODO: Add appropriate meshkit error
		http.Error(rw, ErrGetMeshModels(err).Error(), http.StatusInternalServerError)
	}
}

// swagger:route GET /api/meshmodels/categories GetMeshmodelCategories idGetMeshmodelCategories
// Handle GET request for getting all meshmodel categories
//
// ```?order={field}``` orders on the passed field
//
// ```?sort={[asc/desc]}``` Default behavior is asc
//
// ```?search={categoryName}``` If search is non empty then a greedy search is performed
//
// ```?page={page-number}``` Default page number is 1
//
// ```?pagesize={pagesize}``` Default pagesize is 25. To return all results: ```pagesize=all```
// responses:
//
//	200: []meshmodelCategoriesResponseWrapper
func (h *Handler) GetMeshmodelCategories(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Add("Content-Type", "application/json")
	enc := json.NewEncoder(rw)
	page, offset, limit, search, order, sort, _ := getPaginationParams(r)
	filter := &regv1beta1.CategoryFilter{
		Limit:   limit,
		Offset:  offset,
		OrderOn: order,
		Sort:    sort,
	}
	if search != "" {
		filter.Greedy = true
		filter.Name = search
	}

	categories, count, _, _ := h.registryManager.GetEntities(filter)

	var pgSize int64

	if limit == 0 {
		pgSize = count
	} else {
		pgSize = int64(limit)
	}

	res := models.MeshmodelCategoriesAPIResponse{
		Page:       page,
		PageSize:   int(pgSize),
		Count:      count,
		Categories: categories,
	}

	if err := enc.Encode(res); err != nil {
		h.log.Error(ErrGetMeshModels(err)) //TODO: Add appropriate meshkit error
		http.Error(rw, ErrGetMeshModels(err).Error(), http.StatusInternalServerError)
	}
}

// swagger:route GET /api/meshmodels/categories/{category} GetMeshmodelCategoriesByName idGetMeshmodelCategoriesByName
// Handle GET request for getting all meshmodel categories of a given name
//
// ```?order={field}``` orders on the passed field
//
// ```?sort={[asc/desc]}``` Default behavior is asc
//
// ```?page={page-number}``` Default page number is 1
//
// ```?pagesize={pagesize}``` Default pagesize is 25. To return all results: ```pagesize=all```
//
// ```?search={[true/false]}``` If search is true then a greedy search is performed
// responses:
//
//	200: []meshmodelCategoriesResponseWrapper
func (h *Handler) GetMeshmodelCategoriesByName(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Add("Content-Type", "application/json")
	enc := json.NewEncoder(rw)
	page, offset, limit, search, order, sort, _ := getPaginationParams(r)
	name := mux.Vars(r)["category"]
	var greedy bool
	if search == "true" {
		greedy = true
	}
	categories, count, _, _ := h.registryManager.GetEntities(&regv1beta1.CategoryFilter{
		Name:    name,
		Limit:   limit,
		Greedy:  greedy,
		Offset:  offset,
		OrderOn: order,
		Sort:    sort,
	})

	var pgSize int64

	if limit == 0 {
		pgSize = count
	} else {
		pgSize = int64(limit)
	}

	res := models.MeshmodelCategoriesAPIResponse{
		Page:       page,
		PageSize:   int(pgSize),
		Count:      count,
		Categories: categories,
	}

	if err := enc.Encode(res); err != nil {
		h.log.Error(ErrGetMeshModels(err)) //TODO: Add appropriate meshkit error
		http.Error(rw, ErrGetMeshModels(err).Error(), http.StatusInternalServerError)
	}
}

// swagger:route GET /api/meshmodels/categories/{category}/models/{model}/components/{name} GetMeshmodelComponentsByNameByModelByCategory idGetMeshmodelComponentsByNameByModelByCategory
// Handle GET request for getting meshmodel components of a specific type by model and category.
//
// Example: ```/api/meshmodels/categories/Orchestration``` and Management/models/kubernetes/components/Namespace
// Components can be further filtered through query parameter
//
// ```?version={version}``` If version is unspecified then all model versions are returned
//
// ```?apiVersion={apiVersion}``` If apiVersion is unspecified then all components are returned
//
// ```?order={field}``` orders on the passed field
//
// ```?search={[true/false]}``` If search is true then a greedy search is performed
//
// ```?sort={[asc/desc]}``` Default behavior is asc
//
// ```?page={page-number}``` Default page number is 1
//
// ```?pagesize={pagesize}``` Default pagesize is 25. To return all results: ```pagesize=all```
//
// ```?annotations={["true"/"false"/]}``` If "true" components having "isAnnotation" property as true are "only" returned, If false all components except "annotations" are returned. Any other value of the query parameter results in both annoations as well as non-annotation components being returned.
// responses:
// 200: []meshmodelComponentsDuplicateResponseWrapper
func (h *Handler) GetMeshmodelComponentsByNameByModelByCategory(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Add("Content-Type", "application/json")
	enc := json.NewEncoder(rw)
	page, offset, limit, search, order, sort, _ := getPaginationParams(r)
	name := mux.Vars(r)["name"]

	queryParams := r.URL.Query()
	var greedy bool
	if search == "true" {
		greedy = true
	}
	typ := mux.Vars(r)["model"]
	cat := mux.Vars(r)["category"]
	v := queryParams.Get("version")
	returnAnnotationComp := queryParams.Get("annotations")
	entities, count, _, _ := h.registryManager.GetEntities(&regv1beta1.ComponentFilter{
		Name:         name,
		CategoryName: cat,
		ModelName:    typ,
		APIVersion:   queryParams.Get("apiVersion"),
		Version:      v,
		Offset:       offset,
		Greedy:       greedy,
		Limit:        limit,
		OrderOn:      order,
		Sort:         sort,
		Annotations:  returnAnnotationComp,
	})

	comps := prettifyCompDefSchema(entities)

	var pgSize int64
	if limit == 0 {
		pgSize = count
	} else {
		pgSize = int64(limit)
	}

	response := models.MeshmodelComponentsDuplicateAPIResponse{
		Page:       page,
		PageSize:   int(pgSize),
		Count:      count,
		Components: models.FindDuplicateComponents(comps),
	}

	if err := enc.Encode(response); err != nil {
		h.log.Error(ErrGetMeshModels(err)) //TODO: Add appropriate meshkit error
		http.Error(rw, ErrGetMeshModels(err).Error(), http.StatusInternalServerError)
	}
}

// swagger:route GET /api/meshmodels/categories/{category}/components/{name} GetMeshmodelComponentsByNameByCategory idGetMeshmodelComponentsByNameByCategory
// Handle GET request for getting meshmodel components of a specific type category.
//
// Example: ```/api/meshmodels/categories/Orchestration``` and Management/components/Namespace
// Components can be further filtered through query parameter
//
// ```?model={model}``` If model is unspecified then all models are returned
//
// ```?version={version}``` If version is unspecified then all model versions are returned
//
// ```?apiVersion={apiVersion}``` If apiVersion is unspecified then all components are returned
//
// ```?order={field}``` orders on the passed field
//
// ```?search={[true/false]}``` If search is true then a greedy search is performed
//
// ```?sort={[asc/desc]}``` Default behavior is asc
//
// ```?page={page-number}``` Default page number is 1
//
// ```?pagesize={pagesize}``` Default pagesize is 25. To return all results: ```pagesize=all```
//
// ```?annotations={["true"/"false"/]}``` If "true" components having "isAnnotation" property as true are "only" returned, If false all components except "annotations" are returned. Any other value of the query parameter results in both annoations as well as non-annotation components being returned.
// responses:
//
//	200: []meshmodelComponentsDuplicateResponseWrapper
func (h *Handler) GetMeshmodelComponentsByNameByCategory(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Add("Content-Type", "application/json")
	enc := json.NewEncoder(rw)
	page, offset, limit, search, order, sort, _ := getPaginationParams(r)
	name := mux.Vars(r)["name"]
	var greedy bool
	queryParams := r.URL.Query()
	if search == "true" {
		greedy = true
	}
	cat := mux.Vars(r)["category"]
	v := queryParams.Get("version")
	returnAnnotationComp := queryParams.Get("annotations")

	entities, count, _, _ := h.registryManager.GetEntities(&regv1beta1.ComponentFilter{
		Name:         name,
		ModelName:    queryParams.Get("model"),
		CategoryName: cat,
		APIVersion:   queryParams.Get("apiVersion"),
		Version:      v,
		Offset:       offset,
		Limit:        limit,
		Greedy:       greedy,
		OrderOn:      order,
		Sort:         sort,
		Annotations:  returnAnnotationComp,
	})
	comps := prettifyCompDefSchema(entities)

	var pgSize int64
	if limit == 0 {
		pgSize = count
	} else {
		pgSize = int64(limit)
	}

	response := models.MeshmodelComponentsDuplicateAPIResponse{
		Page:       page,
		PageSize:   int(pgSize),
		Count:      count,
		Components: models.FindDuplicateComponents(comps),
	}

	if err := enc.Encode(response); err != nil {
		h.log.Error(ErrGetMeshModels(err)) //TODO: Add appropriate meshkit error
		http.Error(rw, ErrGetMeshModels(err).Error(), http.StatusInternalServerError)
	}
}

// swagger:route GET /api/meshmodels/models/{model}/components/{name} GetMeshmodelComponentsByNameByModel idGetMeshmodelComponentsByNameByModel
// Handle GET request for getting meshmodel components of a specific  model.
//
// Example: ```/api/meshmodels/models/kubernetes/components/Namespace```
// Components can be further filtered through query parameter
//
// ```?version={version}``` If version is unspecified then all model versions are returned
//
// ```?apiVersion={apiVersion}``` If apiVersion is unspecified then all components are returned
//
// ```?order={field}``` orders on the passed field
//
// ```?sort={[asc/desc]}``` Default behavior is asc
//
// ```?search={[true/false]}``` If search is true then a greedy search is performed
//
// ```?page={page-number}``` Default page number is 1
//
// ```?pagesize={pagesize}``` Default pagesize is 25. To return all results: ```pagesize=all```
//
// ```?annotations={["true"/"false"/]}``` If "true" components having "isAnnotation" property as true are "only" returned, If false all components except "annotations" are returned. Any other value of the query parameter results in both annoations as well as non-annotation components being returned. returned.
// responses:
//
//	200: []meshmodelComponentsDuplicateResponseWrapper
func (h *Handler) GetMeshmodelComponentsByNameByModel(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Add("Content-Type", "application/json")
	enc := json.NewEncoder(rw)
	page, offset, limit, search, order, sort, _ := getPaginationParams(r)
	name := mux.Vars(r)["name"]
	var greedy bool
	queryParams := r.URL.Query()

	if search == "true" {
		greedy = true
	}
	typ := mux.Vars(r)["model"]
	v := queryParams.Get("version")

	returnAnnotationComp := queryParams.Get("annotations")

	entities, count, _, _ := h.registryManager.GetEntities(&regv1beta1.ComponentFilter{
		Name:        name,
		ModelName:   typ,
		APIVersion:  queryParams.Get("apiVersion"),
		Version:     v,
		Offset:      offset,
		Greedy:      greedy,
		Limit:       limit,
		OrderOn:     order,
		Sort:        sort,
		Annotations: returnAnnotationComp,
	})
	comps := prettifyCompDefSchema(entities)

	var pgSize int64
	if limit == 0 {
		pgSize = count
	} else {
		pgSize = int64(limit)
	}

	response := models.MeshmodelComponentsDuplicateAPIResponse{
		Page:       page,
		PageSize:   int(pgSize),
		Count:      count,
		Components: models.FindDuplicateComponents(comps),
	}

	if err := enc.Encode(response); err != nil {
		h.log.Error(ErrGetMeshModels(err)) //TODO: Add appropriate meshkit error
		http.Error(rw, ErrGetMeshModels(err).Error(), http.StatusInternalServerError)
	}
}

// swagger:route GET /api/meshmodels/components/{name} GetAllMeshmodelComponentsByName idGetAllMeshmodelComponentsByName
// Handle GET request for getting meshmodel components of a specific type by name across all models and categories
//
// Example: ```/api/meshmodels/components/Namespace```
// Components can be further filtered through query parameter
//
// ```?model={model}``` If model is unspecified then all models are returned
//
// ```?version={version}``` If version is unspecified then all model versions are returned
//
// ```?apiVersion={apiVersion}``` If apiVersion is unspecified then all components are returned
//
// ```?order={field}``` orders on the passed field
//
// ```?sort={[asc/desc]}``` Default behavior is asc
//
// ```?trim={[true]}``` When trim is set to true, the underlying schemas are not returned for entities
//
// ```?search={[true/false]}``` If search is true then a greedy search is performed
//
// ```?page={page-number}``` Default page number is 1
//
// ```?pagesize={pagesize}``` Default pagesize is 25. To return all results: ```pagesize=all```
//
// ```?annotations={["true"/"false"/]}``` If "true" components having "isAnnotation" property as true are "only" returned, If false all components except "annotations" are returned. Any other value of the query parameter results in both annoations as well as non-annotation components being returned.
// responses:
// 200: []meshmodelComponentsDuplicateResponseWrapper
func (h *Handler) GetAllMeshmodelComponentsByName(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Add("Content-Type", "application/json")
	enc := json.NewEncoder(rw)
	page, offset, limit, search, order, sort, _ := getPaginationParams(r)
	name := mux.Vars(r)["name"]
	var greedy bool
	queryParams := r.URL.Query()
	if search == "true" {
		greedy = true
	}
	v := queryParams.Get("version")
	returnAnnotationComp := queryParams.Get("annotations")
	entities, count, _, _ := h.registryManager.GetEntities(&regv1beta1.ComponentFilter{
		Name:        name,
		Trim:        queryParams.Get("trim") == "true",
		APIVersion:  queryParams.Get("apiVersion"),
		Version:     v,
		ModelName:   queryParams.Get("model"),
		Offset:      offset,
		Limit:       limit,
		Greedy:      greedy,
		OrderOn:     order,
		Sort:        sort,
		Annotations: returnAnnotationComp,
	})

	comps := prettifyCompDefSchema(entities)

	var pgSize int64
	if limit == 0 {
		pgSize = count
	} else {
		pgSize = int64(limit)
	}

	response := models.MeshmodelComponentsDuplicateAPIResponse{
		Page:       page,
		PageSize:   int(pgSize),
		Count:      count,
		Components: models.FindDuplicateComponents(comps),
	}

	if err := enc.Encode(response); err != nil {
		h.log.Error(ErrGetMeshModels(err)) //TODO: Add appropriate meshkit error
		http.Error(rw, ErrGetMeshModels(err).Error(), http.StatusInternalServerError)
	}
}

// swagger:route GET /api/meshmodels/models/{model}/components GetMeshmodelComponentByModel idGetMeshmodelComponentByModel
// Handle GET request for getting meshmodel components of a specific model. The component type/model name should be lowercase like "kubernetes", "istio"
//
// Example: ```/api/meshmodels/models/kubernetes/components```
// Components can be further filtered through query parameter
//
// ```?version={version}```
//
// ```?trim={[true]}``` When trim is set to true, the underlying schemas are not returned for entities
//
// ```?apiVersion={apiVersion}``` If apiVersion is unspecified then all models are returned
//
// ```?search={componentname}``` If search is non empty then a greedy search is performed
//
// ```?order={field}``` orders on the passed field
//
// ```?sort={[asc/desc]}``` Default behavior is asc
//
// ```?page={page-number}``` Default page number is 1
//
// ```?pagesize={pagesize}``` Default pagesize is 25. To return all results: ```pagesize=all```
//
// ```?annotations={["true"/"false"/]}``` If "true" components having "isAnnotation" property as true are "only" returned, If false all components except "annotations" are returned. Any other value of the query parameter results in both annoations as well as non-annotation components being returned.
// responses:
// 200: []meshmodelComponentsDuplicateResponseWrapper
func (h *Handler) GetMeshmodelComponentByModel(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Add("Content-Type", "application/json")
	enc := json.NewEncoder(rw)
	page, offset, limit, search, order, sort, _ := getPaginationParams(r)
	typ := mux.Vars(r)["model"]
	queryParams := r.URL.Query()
	v := queryParams.Get("version")

	returnAnnotationComp := queryParams.Get("annotations")
	filter := &regv1beta1.ComponentFilter{
		Id:          queryParams.Get("id"),
		ModelName:   typ,
		Version:     v,
		Trim:        queryParams.Get("trim") == "true",
		APIVersion:  queryParams.Get("apiVersion"),
		Limit:       limit,
		Offset:      offset,
		OrderOn:     order,
		Sort:        sort,
		Annotations: returnAnnotationComp,
	}
	if search != "" {
		filter.Greedy = true
		filter.DisplayName = search
	}
	entities, count, _, _ := h.registryManager.GetEntities(filter)
	comps := prettifyCompDefSchema(entities)

	var pgSize int64
	if limit == 0 {
		pgSize = count
	} else {
		pgSize = int64(limit)
	}

	response := models.MeshmodelComponentsDuplicateAPIResponse{
		Page:       page,
		PageSize:   int(pgSize),
		Count:      count,
		Components: models.FindDuplicateComponents(comps),
	}

	if err := enc.Encode(response); err != nil {
		h.log.Error(ErrGetMeshModels(err)) //TODO: Add appropriate meshkit error
		http.Error(rw, ErrGetMeshModels(err).Error(), http.StatusInternalServerError)
	}
}

// swagger:route GET /api/meshmodels/categories/{category}/models/{model}/components GetMeshmodelComponentByModelByCategory idGetMeshmodelComponentByModelByCategory
//
// Handle GET request for getting meshmodel components of a specific model and category. The component type/model name should be lowercase like "kubernetes", "istio"
//
// Example: ```/api/meshmodels/categories/Orchestration``` and Management/models/kubernetes/components
// Components can be further filtered through query parameter
//
// ```?version={version}```
//
// ```?trim={[true]}``` When trim is set to true, the underlying schemas are not returned for entities
//
// ```?apiVersion={apiVersion}``` If apiVersion is unspecified then all models are returned
//
// ```?order={field}``` orders on the passed field
//
// ```?search={componentname}``` If search is non empty then a greedy search is performed
//
// ```?sort={[asc/desc]}``` Default behavior is asc
//
// ```?page={page-number}``` Default page number is 1
//
// ```?pagesize={pagesize}``` Default pagesize is 25. To return all results: ```pagesize=all```
//
// ```?annotations={["true"/"false"/]}``` If "true" components having "isAnnotation" property as true are "only" returned, If false all components except "annotations" are returned. Any other value of the query parameter results in both annoations as well as non-annotation components being returned.
// responses:
// 200: []meshmodelComponentsDuplicateResponseWrapper
func (h *Handler) GetMeshmodelComponentByModelByCategory(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Add("Content-Type", "application/json")
	enc := json.NewEncoder(rw)
	page, offset, limit, search, order, sort, _ := getPaginationParams(r)
	typ := mux.Vars(r)["model"]
	cat := mux.Vars(r)["category"]
	queryParams := r.URL.Query()
	v := queryParams.Get("version")
	returnAnnotationComp := queryParams.Get("annotations")
	filter := &regv1beta1.ComponentFilter{
		CategoryName: cat,
		ModelName:    typ,
		Version:      v,
		Trim:         queryParams.Get("trim") == "true",
		APIVersion:   queryParams.Get("apiVersion"),
		Limit:        limit,
		Offset:       offset,
		OrderOn:      order,
		Sort:         sort,
		Annotations:  returnAnnotationComp,
	}
	if search != "" {
		filter.Greedy = true
		filter.DisplayName = search
	}
	entities, count, _, _ := h.registryManager.GetEntities(filter)
	comps := prettifyCompDefSchema(entities)

	var pgSize int64
	if limit == 0 {
		pgSize = count
	} else {
		pgSize = int64(limit)
	}

	response := models.MeshmodelComponentsDuplicateAPIResponse{
		Page:       page,
		PageSize:   int(pgSize),
		Count:      count,
		Components: models.FindDuplicateComponents(comps),
	}

	if err := enc.Encode(response); err != nil {
		h.log.Error(ErrGetMeshModels(err)) //TODO: Add appropriate meshkit error
		http.Error(rw, ErrGetMeshModels(err).Error(), http.StatusInternalServerError)
	}
}

// swagger:route GET /api/meshmodels/categories/{category}/components GetMeshmodelComponentByCategory idGetMeshmodelComponentByCategory
// Handle GET request for getting meshmodel components of a specific model and category.
//
// # Components can be further filtered through query parameter
//
// ```?version={version}```
//
// ```?trim={[true]}``` When trim is set to true, the underlying schemas are not returned for entities
//
// ```?apiVersion={apiVersion}``` If apiVersion is unspecified then all models are returned
//
// ```?order={field}``` orders on the passed field
//
// ```?search={componentname}``` If search is non empty then a greedy search is performed
//
// ```?sort={[asc/desc]}``` Default behavior is asc
//
// ```?page={page-number}``` Default page number is 1
//
// ```?pagesize={pagesize}``` Default pagesize is 25. To return all results: ```pagesize=all```
//
// ```?annotations={["true"/"false"/]}``` If "true" components having "isAnnotation" property as true are "only" returned, If false all components except "annotations" are returned. Any other value of the query parameter results in both annoations as well as non-annotation components being returned.
// responses:
//
//	200: []meshmodelComponentsDuplicateResponseWrapper
func (h *Handler) GetMeshmodelComponentByCategory(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Add("Content-Type", "application/json")
	enc := json.NewEncoder(rw)
	page, offset, limit, search, order, sort, _ := getPaginationParams(r)
	cat := mux.Vars(r)["category"]
	queryParams := r.URL.Query()
	v := queryParams.Get("version")
	returnAnnotationComp := queryParams.Get("annotations")
	filter := &regv1beta1.ComponentFilter{
		CategoryName: cat,
		Version:      v,
		Trim:         queryParams.Get("trim") == "true",
		APIVersion:   queryParams.Get("apiVersion"),
		Limit:        limit,
		Offset:       offset,
		OrderOn:      order,
		Sort:         sort,
		Annotations:  returnAnnotationComp,
	}
	if search != "" {
		filter.Greedy = true
		filter.DisplayName = search
	}
	entities, count, _, _ := h.registryManager.GetEntities(filter)
	comps := prettifyCompDefSchema(entities)

	var pgSize int64
	if limit == 0 {
		pgSize = count
	} else {
		pgSize = int64(limit)
	}

	response := models.MeshmodelComponentsDuplicateAPIResponse{
		Page:       page,
		PageSize:   int(pgSize),
		Count:      count,
		Components: models.FindDuplicateComponents(comps),
	}

	if err := enc.Encode(response); err != nil {
		h.log.Error(ErrGetMeshModels(err)) //TODO: Add appropriate meshkit error
		http.Error(rw, ErrGetMeshModels(err).Error(), http.StatusInternalServerError)
	}
}

// swagger:route GET /api/meshmodels/components GetAllMeshmodelComponents idGetAllMeshmodelComponents
// Handle GET request for getting meshmodel components across all models and categories
//
// # Components can be further filtered through query parameter
//
// ```?version={version}```
//
// ```?apiVersion={apiVersion}``` If apiVersion is unspecified then all models are returned
//
// ```?order={field}``` orders on the passed field
//
// ```?search={componentname}``` If search is non empty then a greedy search is performed
//
// ```?trim={[true]}``` When trim is set to true, the underlying schemas are not returned for entities
//
// ```?sort={[asc/desc]}``` Default behavior is asc
//
// ```?page={page-number}``` Default page number is 1
//
// ```?pagesize={pagesize}``` Default pagesize is 25. To return all results: ```pagesize=all```
//
// ```?annotations={["true"/"false"/]}``` If "true" components having "isAnnotation" property as true are "only" returned, If false all components except "annotations" are returned. Any other value of the query parameter results in both annoations as well as non-annotation components being returned.
// responses:
//  200: meshmodelComponentsDuplicateResponseWrapper

func (h *Handler) GetAllMeshmodelComponents(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Add("Content-Type", "application/json")
	enc := json.NewEncoder(rw)
	page, offset, limit, search, order, sort, _ := getPaginationParams(r)
	queryParams := r.URL.Query()
	v := queryParams.Get("version")
	returnAnnotationComp := queryParams.Get("annotations")
	filter := &regv1beta1.ComponentFilter{
		Id:          queryParams.Get("id"),
		Version:     v,
		Trim:        queryParams.Get("trim") == "true",
		APIVersion:  queryParams.Get("apiVersion"),
		Limit:       limit,
		Offset:      offset,
		OrderOn:     order,
		Sort:        sort,
		Annotations: returnAnnotationComp,
	}
	if search != "" {
		filter.Greedy = true
		filter.DisplayName = search
	}
	entities, count, _, _ := h.registryManager.GetEntities(filter)
	comps := prettifyCompDefSchema(entities)

	var pgSize int64

	if limit == 0 {
		pgSize = count
	} else {
		pgSize = int64(limit)
	}

	res := models.MeshmodelComponentsDuplicateAPIResponse{
		Page:       page,
		PageSize:   int(pgSize),
		Count:      count,
		Components: models.FindDuplicateComponents(comps),
	}

	if err := enc.Encode(res); err != nil {
		h.log.Error(ErrGetMeshModels(err)) //TODO: Add appropriate meshkit error
		http.Error(rw, ErrGetMeshModels(err).Error(), http.StatusInternalServerError)
	}
}

// swagger:route POST /api/meshmodel/components/register MeshmodelValidate idPostMeshModelValidate
// Handle POST request for registering meshmodel components.
//
// Validate the given value with the given schema
// responses:
// 	200:

// request body should be json
// request body should be of ComponentCapability format
func (h *Handler) RegisterMeshmodelComponents(rw http.ResponseWriter, r *http.Request) {
	dec := json.NewDecoder(r.Body)
	var cc registry.MeshModelRegistrantData
	err := dec.Decode(&cc)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}
	var c component.ComponentDefinition
	switch cc.EntityType {
	case entity.ComponentDefinition:
		var isModelError bool
		var isRegistranError bool
		err = json.Unmarshal(cc.Entity, &c)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}
		utils.WriteSVGsOnFileSystem(&c)
		isRegistranError, isModelError, err = h.registryManager.RegisterEntity(cc.Connection, &c)
		helpers.HandleError(cc.Connection, &c, err, isModelError, isRegistranError)
	}
	err = helpers.WriteLogsToFiles()
	if err != nil {
		h.log.Error(err)
	}
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}
	go h.config.MeshModelSummaryChannel.Publish()
}

// swagger:route GET /api/meshmodels/registrants GetMeshmodelRegistrants
// Handle GET request for getting all meshmodel registrants
//
// # Returns a list of registrants and summary count of its models, components, and relationships
//
// ```?page={pagenumber}``` Default page number is 1
//
// ```?order={field}``` orders on the passed field
//
// ```?search={Hostname}``` Gets host by the name
//
// ```?sort={[asc/desc]}``` Default behavior is asc
//
// ```?pagesize={pagesize}``` Default pagesize is 25. To return all results: ```pagesize=all```
//
// responses:
//	200: []meshmodelRegistrantsResponseWrapper

func (h *Handler) GetMeshmodelRegistrants(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Add("Content-Type", "application/json")
	enc := json.NewEncoder(rw)
	page, offset, limit, search, order, sort, _ := getPaginationParams(r)

	filter := &_models.HostFilter{
		Limit:   limit,
		Offset:  offset,
		Sort:    sort,
		OrderOn: order,
	}
	if search != "" {
		filter.Greedy = true
		filter.DisplayName = search
	}
	hosts, count, err := h.registryManager.GetRegistrants(filter)
	if err != nil {
		h.log.Error(ErrGetMeshModels(err))
		http.Error(rw, ErrGetMeshModels(err).Error(), http.StatusInternalServerError)
		return
	}

	var pgSize int64

	if limit == 0 {
		pgSize = count
	} else {
		pgSize = int64(limit)
	}
	res := models.MeshmodelRegistrantsAPIResponse{
		Page:        page,
		PageSize:    int(pgSize),
		Count:       count,
		Registrants: hosts,
	}

	if err := enc.Encode(res); err != nil {
		h.log.Error(ErrGetMeshModels(err))
		http.Error(rw, ErrGetMeshModels(err).Error(), http.StatusInternalServerError)
	}
}

// swagger:route POST /api/meshmodel/update/status MeshModelUpdateEntityStatus idPostMeshModelUpdateEntityStatus
// Handle POST request for updating the ignore status of a model.
//
// Update the ignore status of a model based on the provided parameters.
//
// responses:
// 	200: noContentWrapper

// request body should be json
// request body should be of struct containing ID and Status fields
func (h *Handler) UpdateEntityStatus(rw http.ResponseWriter, r *http.Request, _ *models.Preference, user *models.User, provider models.Provider) {
	dec := json.NewDecoder(r.Body)
	userID := uuid.FromStringOrNil(user.ID)
	entityType := mux.Vars(r)["entityType"]
	var updateData struct {
		ID          string `json:"id"`
		Status      string `json:"status"`
		DisplayName string `json:"displayname"`
	}
	err := dec.Decode(&updateData)
	if err != nil {
		h.log.Error(ErrRequestBody(err))
		http.Error(rw, ErrRequestBody(err).Error(), http.StatusInternalServerError)
		return
	}

	eventBuilder := events.NewEvent().ActedUpon(userID).FromUser(userID).FromSystem(*h.SystemID).WithCategory(entityType).WithAction("update")
	err = h.registryManager.UpdateEntityStatus(updateData.ID, updateData.Status, entityType)
	if err != nil {
		eventBuilder.WithSeverity(events.Error).WithDescription(fmt.Sprintf("Failed to update '%s' status to %s", updateData.DisplayName, updateData.Status)).WithMetadata(map[string]interface{}{
			"error": err,
		})
		_event := eventBuilder.Build()
		_ = provider.PersistEvent(*_event, nil)
		go h.config.EventBroadcaster.Publish(userID, _event)
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	description := fmt.Sprintf("Status of '%s' updated to %s.", updateData.DisplayName, updateData.Status)

	event := eventBuilder.WithSeverity(events.Informational).WithDescription(description).Build()
	_ = provider.PersistEvent(*event, nil)
	go h.config.EventBroadcaster.Publish(userID, event)

	// Respond with success status
	rw.WriteHeader(http.StatusNoContent)
}

func prettifyCompDefSchema(entities []entity.Entity) []component.ComponentDefinition {
	var comps []component.ComponentDefinition
	for _, r := range entities {
		comp, ok := r.(*component.ComponentDefinition)
		if ok {
			m := make(map[string]interface{})
			_ = json.Unmarshal([]byte(comp.Component.Schema), &m)
			m = core.Format.Prettify(m, true)
			b, _ := json.Marshal(m)
			comp.Component.Schema = string(b)
			comps = append(comps, *comp)
		}
	}
	return comps
}

// swagger:route POST /api/meshmodels/register RegisterMeshmodels idRegisterMeshmodels
// Handle POST request for registering entites like components and relationships model.
//
// Register model based on thier Schema Version.
//
// responses:
// 	200: noContentWrapper

// request content byte in form value and header of the type in form

func (h *Handler) RegisterMeshmodels(rw http.ResponseWriter, r *http.Request, _ *models.Preference, user *models.User, provider models.Provider) {
	var response models.RegistryAPIResponse
	regErrorStore := models.NewRegistrationFailureLogHandler()
	var tempFile *os.File
	var mu sync.Mutex
	defer func() {
		_ = r.Body.Close()
	}()
	userID := uuid.FromStringOrNil(user.ID)
	var message string

	//Here the codes handles to decode and store the data from the payload
	var importRequest schemav1beta1.ImportRequest

	err := json.NewDecoder(r.Body).Decode(&importRequest)
	if err != nil {
		h.log.Info("Error in unmarshalling request body")
		h.sendErrorEvent(userID, provider, "Error in unmarshalling request body", err)
		http.Error(rw, "Invalid request format", http.StatusBadRequest)
		return
	}

	registrationHelper := registration.NewRegistrationHelper(
		utils.UI,
		h.registryManager,
		regErrorStore,
	)
	var dir registration.Dir
	switch importRequest.UploadType {
	case "csv":
		err := meshkitRegistryUtils.SetLogger(false)
		if err != nil {
			h.handleError(rw, err, "Error setting logger")
			h.sendErrorEvent(userID, provider, "Error setting logger", err)
		}
		fetchBase64DataFromDataURL := func(dataURL string) ([]byte, error) {
			if strings.HasPrefix(dataURL, "data:text/csv;base64,") {
				base64Data := strings.TrimPrefix(dataURL, "data:text/csv;base64,")
				return base64.StdEncoding.DecodeString(base64Data)
			}
			return nil, ErrFileType("file is not of type csv")
		}
		modelCSVData, err := fetchBase64DataFromDataURL(importRequest.ImportBody.ModelCsv)
		if err != nil {
			h.handleError(rw, err, "Error fetching or decoding Model CSV")
			h.sendErrorEvent(userID, provider, "Error fetching or decoding Model CSV", err)
			return
		}
		modelCsvFile, err := os.CreateTemp("", "model-*.csv")
		if err != nil {
			err = ErrCreateFile(err, "Error creating temp file for Model CSV")
			h.handleError(rw, err, "Error creating temp file for Model CSV")
			h.sendErrorEvent(userID, provider, "Error creating temp file for Model CSV", err)
			return
		}
		defer modelCsvFile.Close()

		_, err = modelCsvFile.Write(modelCSVData)
		if err != nil {
			err = ErrWritingIntoFile(err, "Error writing Model CSV to temp file")
			h.handleError(rw, err, "Error writing Model CSV to temp file")
			h.sendErrorEvent(userID, provider, "Error writing Model CSV to temp file", err)
			return
		}

		componentCSVData, err := fetchBase64DataFromDataURL(importRequest.ImportBody.ComponentCsv)
		if err != nil {
			h.handleError(rw, err, "Error fetching or decoding Component CSV")
			h.sendErrorEvent(userID, provider, "Error fetching or decoding Component CSV", err)
			return
		}

		componentCsvFile, err := os.CreateTemp("", "component-*.csv")
		if err != nil {
			err = ErrCreateFile(err, "Error creating temp file for Component CSV")
			h.handleError(rw, err, "Error creating temp file for Component CSV")
			h.sendErrorEvent(userID, provider, "Error creating temp file for Component CSV", err)
			return
		}
		defer componentCsvFile.Close()

		_, err = componentCsvFile.Write(componentCSVData)
		if err != nil {
			err = ErrWritingIntoFile(err, "Error writing Component CSV to temp file")
			h.handleError(rw, err, "Error writing Component CSV to temp file")
			h.sendErrorEvent(userID, provider, "Error writing Component CSV to temp file", err)
			return
		}

		relationshipCSVData, err := fetchBase64DataFromDataURL(importRequest.ImportBody.RelationshipCSV)
		if err != nil {
			h.handleError(rw, err, "Error fetching or decoding Model CSV")
			h.sendErrorEvent(userID, provider, "Error fetching or decoding Model CSV", err)
			return
		}
		relationshipCsvFile, err := os.CreateTemp("", "relationship-*.csv")
		if err != nil {
			err = ErrCreateFile(err, "Error creating temp file for Model CSV")
			h.handleError(rw, err, "Error creating temp file for Model CSV")
			h.sendErrorEvent(userID, provider, "Error creating temp file for Model CSV", err)
			return
		}
		defer relationshipCsvFile.Close()

		_, err = relationshipCsvFile.Write(relationshipCSVData)
		if err != nil {
			err = ErrWritingIntoFile(err, "Error writing Model CSV to temp file")
			h.handleError(rw, err, "Error writing Model CSV to temp file")
			h.sendErrorEvent(userID, provider, "Error writing Model CSV to temp file", err)
			return
		}

		var wg sync.WaitGroup
		modelLocation := filepath.Join(os.Getenv("HOME"), utils.RegistryLocation)
		if _, err := os.Stat(modelLocation); os.IsNotExist(err) {
			_ = os.MkdirAll(modelLocation, 0755)
		}

		tempDir, err := os.MkdirTemp("", "tempData")
		if err != nil {
			h.handleError(rw, err, "Error creating temporary directory")
			h.sendErrorEvent(userID, provider, "Error creating temporary directory", err)
			return
		}
		defer os.RemoveAll(tempDir)

		err = meshkitRegistryUtils.InvokeGenerationFromSheet(&wg, tempDir, 0, 0, "", "", modelCsvFile.Name(), componentCsvFile.Name(), "", relationshipCsvFile.Name(), 0, nil)
		if err != nil {
			h.handleError(rw, err, "Error invoking generation from sheet")
			h.sendErrorEvent(userID, provider, "Error invoking generation from sheet", err)
			return
		}

		h.sendEventForImport(userID, provider, 0, "", true)
		modelDirPaths, err := models.GetModelDirectoryPaths(tempDir)
		if err != nil {
			h.log.Error(models.ErrSeedingComponents(err))
		}
		if importRequest.Register {
			for _, dirPath := range modelDirPaths {
				dir := registration.NewDir(dirPath)
				registrationHelper.Register(dir)
			}
		} else {
			return
		}

		if _, err := os.Stat(modelLocation); os.IsNotExist(err) {
			_ = os.MkdirAll(modelLocation, 0755)
		}

		err = utils.CopyDirectory(tempDir, modelLocation)
		if err != nil {
			h.handleError(rw, err, "Error copying data to model location")
			h.sendErrorEvent(userID, provider, "Error copying data to model location", err)
			return
		}

	//Case when it is URL and them the model is generated from the URL
	case "url":

		model := &meshkitRegistryUtils.ModelCSV{
			Model:             importRequest.ImportBody.Model.Model,
			ModelDisplayName:  importRequest.ImportBody.Model.ModelDisplayName,
			PrimaryColor:      importRequest.ImportBody.Model.PrimaryColor,
			SecondaryColor:    importRequest.ImportBody.Model.SecondaryColor,
			Category:          importRequest.ImportBody.Model.Category,
			Registrant:        importRequest.ImportBody.Model.Registrant,
			Shape:             importRequest.ImportBody.Model.Shape,
			SubCategory:       importRequest.ImportBody.Model.SubCategory,
			SVGColor:          importRequest.ImportBody.Model.SvgColor,
			SVGWhite:          importRequest.ImportBody.Model.SvgWhite,
			SVGComplete:       importRequest.ImportBody.Model.SvgComplete,
			IsAnnotation:      strconv.FormatBool(importRequest.ImportBody.Model.IsAnnotation),
			PublishToRegistry: strconv.FormatBool(importRequest.ImportBody.Model.PublishToRegistry),
		}
		setDefaultValues(model)
		//Model generation strats from here
		model.Model = strings.ToLower(model.Model)

		pkg, version, err := meshkitRegistryUtils.GenerateModels(model.Registrant, importRequest.ImportBody.Url, model.Model)
		if err != nil {
			h.handleError(rw, err, "Error generating model")
			h.sendErrorEvent(userID, provider, "Error generating model", err)
			return
		}
		modelDirPath, compDirPath, err := utils.CreateVersionedDirectoryForModelAndComp(version, model.Model)
		if err != nil {
			h.handleError(rw, err, "Error decoding JSON into ModelCSV")
			h.sendErrorEvent(userID, provider, "Error decoding JSON into ModelCSV", err)
			return
		}
		filePath := filepath.Join(modelDirPath, model.Model+".json")
		modelDef := model.CreateModelDefinition(version, utils.DefVersion)
		err = modelDef.WriteModelDefinition(filePath, "json")
		if err != nil {
			h.handleError(rw, err, "Error decoding JSON into ModelCSV")
			h.sendErrorEvent(userID, provider, "Error decoding JSON into ModelCSV", err)
			return
		}

		//Component generation starts here
		lengthofComps, _, err := meshkitRegistryUtils.GenerateComponentsFromPkg(pkg, compDirPath, utils.DefVersion, modelDef, model.Group)
		if err != nil {
			h.handleError(rw, err, "Error generating components")
			h.sendErrorEvent(userID, provider, "Error generating components", err)
			return
		}

		//Event when the URL is used to show that we g
		h.sendEventForImport(userID, provider, lengthofComps, model.Model, false)
		if importRequest.Register {
			dir = registration.NewDir(modelDirPath)
			registrationHelper.Register(dir)
		} else {
			return
		}

	case "file":
		base64Data, err := json.Marshal(importRequest.ImportBody.ModelFile)
		if err != nil {
			http.Error(rw, "Internal server error", http.StatusInternalServerError)
			return
		}
		base64String := string(base64Data)
		// Remove double quotes
		base64String = base64String[1 : len(base64String)-1]

		decodedBytes, err := base64.StdEncoding.DecodeString(base64String)
		if err != nil {
			http.Error(rw, "Invalid base64 data", http.StatusBadRequest)
			return
		}
		tempFile, err = CreateTemp(importRequest.ImportBody.FileName, decodedBytes)
		if err != nil {
			err = meshkitutils.ErrCreateFile(err, "Error creating temp file")
			h.handleError(rw, err, err.Error())
			h.sendErrorEvent(userID, provider, "Error creating temp file", err)
			return
		}
		defer os.Remove(tempFile.Name())

		dir = registration.NewDir(tempFile.Name())
		if importRequest.Register {
			registrationHelper.Register(dir)
			tempFile.Close()
		}
	case "urlImport":
		downloadFile := func(url string) ([]byte, error) {
			resp, err := http.Get(url)
			if err != nil {
				return nil, fmt.Errorf("error downloading file from URL: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return nil, fmt.Errorf("failed to download file. Status code: %d", resp.StatusCode)
			}

			fileData, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil, fmt.Errorf("error reading downloaded file: %v", err)
			}

			return fileData, nil
		}

		// Download the file from the provided URL
		fileData, err := downloadFile(importRequest.ImportBody.Url)
		if err != nil {
			h.handleError(rw, err, "Error downloading file from URL")
			h.sendErrorEvent(userID, provider, "Error downloading file from URL", err)
			return
		}
		isOCI := meshkitOci.IsOCIArtifact(fileData)
		fileType := ".tar"
		if !isOCI {
			fileType = detectFileType(fileData)
		}
		name := "model" + fileType
		//write the file to a temp file
		tempFile, err = CreateTemp(name, fileData)
		if err != nil {
			err = meshkitutils.ErrCreateFile(err, "Error creating temp file")
			h.handleError(rw, err, "Error creating temp file")
			h.sendErrorEvent(userID, provider, "Error creating temp file", err)
			return
		}
		defer os.Remove(tempFile.Name())

		dir = registration.NewDir(tempFile.Name())
		if importRequest.Register {
			registrationHelper.Register(dir)
			tempFile.Close()
		}
	}

	h.handleRegistrationAndError(registrationHelper, &mu, &response, regErrorStore)
	var errMsg string
	message = writeMessageString(&response)
	if response.EntityCount.TotalErrCount > 0 {
		errMsg = ErrMsgContruct(&response)
	}

	h.sendSuccessResponse(rw, userID, provider, message, errMsg, &response)

}

// swagger:route GET /api/meshmodels/export ExportModel idExportModel
// Handle GET request for exporting a model.
//
// # Export model with the given id in the output format specified
//
// ```?id={id}```
// ```?output_format={output_format}``` Can be `json`, `yaml`, or `oci`. Default is `oci`
//
// responses:
//
//	200: []byte

func (h *Handler) ExportModel(rw http.ResponseWriter, r *http.Request) {
	modelId := r.URL.Query().Get("id")
	name := r.URL.Query().Get("name")
	version := r.URL.Query().Get("version")
	outputFormat := r.URL.Query().Get("output_format")
	fileTypes := r.URL.Query().Get("file_type")
	if fileTypes == "" {
		fileTypes = "oci"
	}
	if outputFormat == "" {
		outputFormat = "json"
	}
	hasComponents, err := strconv.ParseBool(r.URL.Query().Get("components"))
	if err != nil {
		hasComponents = true
	}

	hasRelationships, err := strconv.ParseBool(r.URL.Query().Get("relationships"))
	if err != nil {
		hasRelationships = true
	}

	// 1. Get the model data
	modelFilter := &regv1beta1.ModelFilter{
		Id:            modelId,
		Name:          name,
		Components:    hasComponents,
		Relationships: hasRelationships,
		Greedy:        true,
		Version:       version,
	}
	e, _, _, err := h.registryManager.GetEntities(modelFilter)
	if err != nil {
		h.log.Error(ErrGetMeshModels(err))
		http.Error(rw, ErrGetMeshModels(err).Error(), http.StatusInternalServerError)
		return
	}

	// No entities found for the given filter
	// This can happen if the model is not found or if the model is not registered
	// or if the model is not in the registry
	// In this case, we return a 404 error
	if len(e) == 0 {
		message := "model with "
		if modelId != "" {
			message += fmt.Sprintf("id %s ", modelId)
		}
		if name != "" {
			message += fmt.Sprintf("name %s ", name)
		}
		if version != "" {
			message += fmt.Sprintf("version %s ", version)
		}
		message += "has not been found"
		// h.log.Error(ErrGetMeshModels(err))
		// http.Error(rw, ErrGetMeshModels(err).Error(), http.StatusNotFound)
		rw.WriteHeader(http.StatusNotFound)
		// rw.Write([]byte(message))
		fmt.Fprintln(rw, message)
		return
	}

	model := e[0].(*_model.ModelDefinition)
	//This path is used to so that the function can be aware of where the svg file is
	//This is for relative path as we are inside meshery/server/cmd/main.go
	//File is stored in Ui folder so we need to move 2 directories back
	//We do this because we use this in mesheryctl as well
	err = model.ReplaceSVGData("../../")
	if err != nil {
		h.log.Error(err)
	}
	// 2. Convert it to oci
	temp := os.TempDir()
	modelDir := filepath.Join(temp, model.Name)
	versionDir := filepath.Join(modelDir, model.Model.Version, model.Version)

	// Create necessary directories: {modelName}/v1.0.0/1.0.0/{components, relationships}
	dirs := []string{
		versionDir,
		filepath.Join(versionDir, "components"),
		filepath.Join(versionDir, "relationships"),
	}

	for _, dir := range dirs {
		err = os.MkdirAll(dir, 0700)
		if err != nil {
			err = meshkitutils.ErrCreateDir(err, "Error creating temp directory")
			h.log.Error(err)
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	defer os.RemoveAll(modelDir)

	components := []component.ComponentDefinition{}
	// Components can be nil if hasComponents is false
	if model.Components != nil {
		components = model.Components.([]component.ComponentDefinition)
	}

	relationships := []relationship.RelationshipDefinition{}
	// Relationships can be nil if hasRelationships is false
	if model.Relationships != nil {
		relationships = model.Relationships.([]relationship.RelationshipDefinition)
	}

	model.Relationships = nil
	model.Components = nil
	// Write model.json to {modelname}/v1.0.0/1.0.0/model.json
	err = model.WriteModelDefinition(filepath.Join(versionDir, fmt.Sprintf("model.%s", outputFormat)), outputFormat)
	if err != nil {
		h.log.Error(err)
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	componentsDir := filepath.Join(versionDir, "components")
	relationshipsDir := filepath.Join(versionDir, "relationships")

	for _, comp := range components {
		_ = comp.ReplaceSVGData("../../")
		comp.Model = *model
		_, err := comp.WriteComponentDefinition(componentsDir, outputFormat)
		if err != nil {
			h.log.Error(err)
		}

	}
	for _, rel := range relationships {
		rel.Model = *model
		err := rel.WriteRelationshipDefinition(relationshipsDir, outputFormat)
		if err != nil {
			h.log.Error(err)
		}

	}

	// Write components into {modelname}/v1.0.0/1.0.0/components

	// At this point, the data has been written to the directories:
	// {modelname}/v1.0.0/1.0.0/model.json
	// {modelname}/v1.0.0/1.0.0/components/*.json
	// {modelname}/v1.0.0/1.0.0/relationships/*.json

	// Build OCI image for the model from the modelDir
	var tarfileName string
	var byt []byte
	if fileTypes == "oci" {
		img, err := meshkitOci.BuildImage(modelDir)
		if err != nil {
			h.log.Error(err) // TODO: Add appropriate meshkit error
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}

		// Save OCI artifact into a tar file
		tarfileName := filepath.Join(modelDir, "model.tar")
		err = meshkitOci.SaveOCIArtifact(img, tarfileName, model.Name)
		if err != nil {
			h.log.Error(err)
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}

		// 3. Send response
		byt, _ = os.ReadFile(tarfileName)
		rw.Header().Add("Content-Type", "application/x-tar")
		rw.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.tar\"", model.Name))
		rw.Header().Set("Content-Length", fmt.Sprintf("%d", len(byt)))
	} else {
		var tarData bytes.Buffer
		err := meshkitutils.Compress(modelDir, &tarData)
		if err != nil {
			h.log.Error(err)
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}
		tarfileName = filepath.Join(modelDir, "model.tar.gz")
		err = os.WriteFile(tarfileName, tarData.Bytes(), 0644)
		if err != nil {
			h.log.Error(err)
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}
		byt, _ = os.ReadFile(tarfileName)
		rw.Header().Add("Content-Type", "application/gzip")
		rw.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.tar.gz\"", model.Name))

	}

	// 3. Send response
	rw.Header().Set("Content-Length", fmt.Sprintf("%d", len(byt)))
	_, err = rw.Write(byt)
	if err != nil {
		h.log.Error(ErrGetMeshModels(err))
		http.Error(rw, ErrGetMeshModels(err).Error(), http.StatusInternalServerError)
	}
}

func RegisterEntity(content []byte, entityType entity.EntityType, h *Handler) error {
	switch entityType {
	case entity.ComponentDefinition:
		var c component.ComponentDefinition
		err := json.Unmarshal(content, &c)
		if err != nil {
			return meshkitutils.ErrUnmarshal(err)
		}
		isRegistrantError, isModelError, err := h.registryManager.RegisterEntity(connection.Connection{
			Kind: c.Model.Registrant.Kind,
		}, &c)
		helpers.HandleError(connection.Connection{
			Kind: c.Model.Registrant.Kind,
		}, &c, err, isModelError, isRegistrantError)
		return nil
	case entity.RelationshipDefinition:
		var r relationship.RelationshipDefinition
		err := json.Unmarshal(content, &r)
		if err != nil {
			return meshkitutils.ErrUnmarshal(err)
		}
		isRegistrantError, isModelError, err := h.registryManager.RegisterEntity(connection.Connection{
			Kind: r.Model.Registrant.Kind,
		}, &r)
		helpers.HandleError(connection.Connection{
			Kind: r.Model.Registrant.Kind,
		}, &r, err, isModelError, isRegistrantError)
		return nil
	}
	return meshkitutils.ErrInvalidSchemaVersion
}

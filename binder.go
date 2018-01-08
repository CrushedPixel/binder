package binder

import (
	"errors"
	"github.com/crushedpixel/margo"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"gopkg.in/go-playground/validator.v8"
	"io"
	"net/http"
	"reflect"
)

const (
	queryParamsKey = "__binderQueryParams"
	bodyParamsKey  = "__binderBodyParams"
)

type Binder interface {
	// Binding returns the Binding to use when binding
	// request parameters into an instance of this type.
	// Binding should always return the same value.
	Binding() binding.Binding
}

// A BindingEndpoint is a margo.Endpoint providing support for
// query and body parameter binding.
type BindingEndpoint struct {
	margo.Endpoint

	// Type of query parameters for parsing and validation.
	// If nil, query parameters are not parsed and validated.
	queryParamsType reflect.Type
	// Type of body parameters for parsing and validation.
	// If nil, body parameters are not parsed and validated.
	bodyParamsType reflect.Type
}

// NewBindingEndpoint returns a new BindingEndpoint for a given HTTP method and URL path,
// with at least one HandlerFunc to be executed when the Endpoint is called.
//
// Panics if no HandlerFunc is provided.
func NewBindingEndpoint(method string, path string, handlers ...margo.HandlerFunc) *BindingEndpoint {
	return &BindingEndpoint{
		Endpoint: margo.NewEndpoint(method, path, handlers...),
	}
}

// GET returns a new GET BindingEndpoint for a path and at least one HandlerFunc.
func GET(path string, handlers ...margo.HandlerFunc) *BindingEndpoint {
	return NewBindingEndpoint(http.MethodGet, path, handlers...)
}

// POST returns a new POST BindingEndpoint for a path and at least one HandlerFunc.
func POST(path string, handlers ...margo.HandlerFunc) *BindingEndpoint {
	return NewBindingEndpoint(http.MethodPost, path, handlers...)
}

// PUT returns a new PUT BindingEndpoint for a path and at least one HandlerFunc.
func PUT(path string, handlers ...margo.HandlerFunc) *BindingEndpoint {
	return NewBindingEndpoint(http.MethodPut, path, handlers...)
}

// PATCH returns a new PATCH BindingEndpoint for a path and at least one HandlerFunc.
func PATCH(path string, handlers ...margo.HandlerFunc) *BindingEndpoint {
	return NewBindingEndpoint(http.MethodPatch, path, handlers...)
}

// DELETE returns a new DELETE BindingEndpoint for a path and at least one HandlerFunc.
func DELETE(path string, handlers ...margo.HandlerFunc) *BindingEndpoint {
	return NewBindingEndpoint(http.MethodDelete, path, handlers...)
}

func (e *BindingEndpoint) Handlers() margo.HandlerChain {
	// construct binding middleware if needed
	var middleware []margo.HandlerFunc
	if e.queryParamsType != nil {
		middleware = append(middleware, bindingMiddleware(e.queryParamsType, queryParamsKey, binding.Query))
	}
	if e.bodyParamsType != nil {
		middleware = append(middleware, bindingMiddleware(e.bodyParamsType, bodyParamsKey, binding.JSON))
	}
	// prepend binding middleware to handlers
	return margo.HandlerChain(append(middleware, e.Endpoint.Handlers()...))
}

// SetQueryParamsModel sets the type to bind request query parameters into.
// If the model type implements Binder, the binding.Binding returned by Binding() is
// used when binding.
// For more information on model definition, refer to https://github.com/gin-gonic/gin#model-binding-and-validation.
//
// The parsed query parameters can be retrieved from the Context in a HandlerFunc using binder.QueryParams(context).
//
// If model is nil, query parameters are not parsed and validated.
// Panics if model is not a struct instance.
//
// Returns self to allow for method chaining.
func (e *BindingEndpoint) SetQueryParamsModel(model interface{}) *BindingEndpoint {
	if model == nil {
		e.queryParamsType = nil
	} else {
		typ := reflect.TypeOf(model)
		if typ.Kind() != reflect.Struct {
			panic(errors.New("query parameter model type must be a struct type"))
		}
		e.queryParamsType = typ
	}
	return e
}

// SetBodyParamsModel sets the type to bind request body parameters into.
// If the model type implements Binder, the binding.Binding returned by Binding() is
// used when binding.
// For more information on model definition, refer to https://github.com/gin-gonic/gin#model-binding-and-validation.
//
// The parsed query parameters can be retrieved from the Context in a HandlerFunc using binder.BodyParams(context).
//
// If model is nil, query parameters are not parsed and validated.
// Panics if model is not a struct instance.
//
// Returns self to allow for method chaining.
func (e *BindingEndpoint) SetBodyParamsModel(model interface{}) *BindingEndpoint {
	if model == nil {
		e.bodyParamsType = nil
	} else {
		typ := reflect.TypeOf(model)
		if typ.Kind() != reflect.Struct {
			panic(errors.New("body parameter model type must be a struct type"))
		}
		e.bodyParamsType = typ
	}
	return e
}

// bindingMiddleware returns a HandlerFunc binding request parameters
// into the specified type and storing it in the context with the specified key.
// If the type implements Binder, it uses the Binding returned by Binding(), otherwise
// it uses defaultBinding.
func bindingMiddleware(typ reflect.Type, key string, defaultBinding binding.Binding) margo.HandlerFunc {
	return func(c *gin.Context) margo.Response {
		instance := reflect.New(typ).Interface()

		b := defaultBinding
		if binder, ok := instance.(Binder); ok {
			b = binder.Binding()
		}

		if err := c.ShouldBindWith(instance, b); err != nil {
			var errs []*bindingError

			// ValidationErrors is a map[string]*FieldError
			if ve, ok := err.(validator.ValidationErrors); ok {
				for _, val := range ve {
					errs = append(errs, newBindingError(val.Name, val.ActualTag))
				}
			} else {
				if err == io.EOF {
					errs = append(errs, newBindingError("", ""))
				} else {
					panic(err)
				}
			}

			return newErrorResponse(http.StatusBadRequest, errs...)
		}

		c.Set(key, instance)
		return nil
	}
}

// bindingError is a struct type used internally to
// represent binding errors for the user.
type bindingError struct {
	Status int
	Field  string `json:"field"`
	Detail string `json:"detail"`
}

func newBindingError(field string, detail string) *bindingError {
	return &bindingError{
		Field:  field,
		Detail: detail,
	}
}

func newErrorResponse(status int, errors ...*bindingError) margo.Response {
	return margo.JSON(status, gin.H{"errors": errors})
}

// BodyParams returns a pointer to the model instance bound to context by a BindingEndpoint.
// Returns nil if no body parameter binding was done.
func BodyParams(context *gin.Context) interface{} {
	p, _ := context.Get(bodyParamsKey)
	return p
}

// QueryParams returns a pointer to the model instance bound to context by a BindingEndpoint.
// Returns nil if no query parameter binding was done.
func QueryParams(context *gin.Context) interface{} {
	p, _ := context.Get(queryParamsKey)
	return p
}

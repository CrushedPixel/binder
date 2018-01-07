/*
Package binder provides request parameter binding for the web framework margo.

Basic example:
	// Gin binding model.
	// For detailed information on model definition,
	// refer to https://github.com/gin-gonic/gin#model-binding-and-validation.
	type ExampleBodyParams struct {
		Message string `json:"message" binding:"required,max=500"`
	}

	func main() {
		app := margo.NewApplication()

		endpoint := binder.POST("/messages", func(c *gin.Context) margo.Response {
			// parsed body params can be retrieved in handler
			// using BodyParams method
			bodyParams := binder.QueryParams(c).(*ExampleBodyParams)

			// do something with body parameters,
			// for example send them back to the user via json
			return margo.JSON200(bodyParams)
		}).SetBodyParamsModel(ExampleBodyParams{}) // set the expected body params model

		app.Endpoint(endpoint)
		app.Run(":8080")
	}
 */
package binder

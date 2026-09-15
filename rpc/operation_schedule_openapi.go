package rpc

// AddOperationScheduleOpenAPI adds the reusable generated-operation scheduling
// contract to a Clicky OpenAPI document.
func AddOperationScheduleOpenAPI(spec *OpenAPISpec) {
	if spec.Paths == nil {
		spec.Paths = map[string]OpenAPIPath{}
	}
	spec.Paths["/api/v1/schedules"] = OpenAPIPath{
		"get": operationScheduleOpenAPIOperation(
			"list_operation_schedules", "List operation schedules", nil,
			"200", "Operation schedules", &OpenAPISchema{Type: "array", Items: operationScheduleSchema()},
		),
		"post": operationScheduleOpenAPIOperation(
			"create_operation_schedule", "Create an operation schedule", operationScheduleInputSchema(),
			"201", "Operation schedule created", operationScheduleSchema(),
		),
	}
	spec.Paths["/api/v1/schedules/{id}"] = OpenAPIPath{
		"put": operationScheduleOpenAPIOperation(
			"update_operation_schedule", "Update an operation schedule", operationScheduleInputSchema(),
			"200", "Operation schedule updated", operationScheduleSchema(), scheduleIDParameter(),
		),
		"delete": operationScheduleOpenAPIOperation(
			"delete_operation_schedule", "Delete an operation schedule", nil,
			"204", "Operation schedule deleted", nil, scheduleIDParameter(),
		),
	}
	spec.Paths["/api/v1/schedules/{id}/run"] = OpenAPIPath{
		"post": operationScheduleOpenAPIOperation(
			"run_operation_schedule", "Run a saved operation schedule", nil,
			"202", "Operation run accepted", operationScheduleRunSchema(), scheduleIDParameter(),
		),
	}
	spec.Paths["/api/v1/schedules/run-now"] = OpenAPIPath{
		"post": operationScheduleOpenAPIOperation(
			"run_scheduled_operation_now", "Run a schedulable operation now", operationRunInputSchema(),
			"202", "Operation run accepted", operationScheduleRunSchema(),
		),
	}
}

func operationScheduleOpenAPIOperation(
	id string,
	summary string,
	body *OpenAPISchema,
	status string,
	description string,
	response *OpenAPISchema,
	parameters ...OpenAPIParameter,
) OpenAPIOperation {
	operation := OpenAPIOperation{
		Tags: []string{"Schedules"}, Summary: summary, OperationID: id, Parameters: parameters,
		Responses: map[string]OpenAPIResponse{status: {Description: description}},
	}
	if body != nil {
		operation.RequestBody = &OpenAPIRequestBody{
			Required: true,
			Content:  map[string]OpenAPIMediaType{"application/json": {Schema: body}},
		}
	}
	if response != nil {
		operation.Responses[status] = OpenAPIResponse{
			Description: description,
			Content:     map[string]OpenAPIMediaType{"application/json": {Schema: response}},
		}
	}
	return operation
}

func operationScheduleInputSchema() *OpenAPISchema {
	return &OpenAPISchema{
		Type:     "object",
		Required: []string{"name", "operationId", "args", "cron", "enabled"},
		Properties: map[string]*OpenAPISchema{
			"name":        {Type: "string"},
			"operationId": {Type: "string"},
			"args":        {Type: "object", AdditionalProperties: &OpenAPISchema{}},
			"cron":        {Type: "string"},
			"timezone":    {Type: "string"},
			"enabled":     {Type: "boolean"},
		},
	}
}

func operationRunInputSchema() *OpenAPISchema {
	return &OpenAPISchema{
		Type: "object", Required: []string{"operationId", "args"},
		Properties: map[string]*OpenAPISchema{
			"operationId": {Type: "string"},
			"args":        {Type: "object", AdditionalProperties: &OpenAPISchema{}},
		},
	}
}

func operationScheduleSchema() *OpenAPISchema {
	input := operationScheduleInputSchema()
	properties := map[string]*OpenAPISchema{
		"id":      {Type: "string", Format: "uuid"},
		"lastRun": {Type: "string", Format: "date-time"},
		"nextRun": {Type: "string", Format: "date-time"},
	}
	for name, property := range input.Properties {
		properties[name] = property
	}
	return &OpenAPISchema{
		Type: "object", Required: append([]string{"id"}, input.Required...), Properties: properties,
	}
}

func operationScheduleRunSchema() *OpenAPISchema {
	return &OpenAPISchema{
		Type: "object", Required: []string{"runId"},
		Properties: map[string]*OpenAPISchema{"runId": {Type: "string", Format: "uuid"}},
	}
}

func scheduleIDParameter() OpenAPIParameter {
	return OpenAPIParameter{
		Name: "id", In: "path", Required: true,
		Description: "Stable operation schedule ID.", Schema: &OpenAPISchema{Type: "string", Format: "uuid"},
	}
}

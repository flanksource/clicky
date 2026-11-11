# Interactive command arguments

Call `rpc.InstallInteractive(root)` after constructing the Cobra command tree. `command -i` opens the operation's JSON Schema form, prefills supplied flags and positional arguments, validates answers, then runs the command. Cancellation returns an error before the command runs. The prompt manager presents the same form through its terminal or web sink.

Attach an existing argument schema with `flags.SetSchema(command.Flags().Lookup("connection"), schema)`. Its type must match the flag's wire type. Labels, constraints and extensions such as `x-clicky-lookup` survive operation conversion, OpenAPI request bodies and `rpc.OperationInputSchema`. For JSON encoded string flags, describe the encoded representation rather than attaching an incompatible object type.

Register a Cobra completion function for terminal choices. Return `cobra.ShellCompDirectiveNoFileComp` for an empty choice set as well as a populated one. The form retains that completion so an empty store still has a lookup. File and directory completions remain handled separately. A remote `x-clicky-lookup` descriptor supplies the web endpoint; terminal choices use the matching completion without replacing that endpoint in the shared schema.

The runnable `examples/interactive_form.go` example exercises the shared form. Run `go run ./examples/interactive_form.go deploy checkout -i --env staging` from the Clicky checkout.

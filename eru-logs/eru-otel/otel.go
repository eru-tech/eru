package eru_otel

/*
	func TracerInit() (*sdktrace.TracerProvider, error) {
		exporter, err := stdout.New(stdout.WithPrettyPrint())
		if err != nil {
			return nil, err
		}
		tp := sdktrace.NewTracerProvider(
			sdktrace.WithSampler(sdktrace.AlwaysSample()),
			sdktrace.WithBatcher(exporter),
		)
		otel.SetTracerProvider(tp)
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
		return tp, nil
	}

func TracerInit() error {
	shutdown := initProvider()
	defer shutdown()

	//meter := global.Meter("demo-server-meter")
	serverAttribute := attribute.String("server-attribute", "foo")
	fmt.Println("start to gen chars for trace data")
	initTraceDemoData()
	fmt.Println("gen trace data done")
	tracer := otel.Tracer(common.TraceInstrumentationName)

	// Create a handler in OpenTelemetry.
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// Simulate a delay.
		var sleep int64
		switch modulus := time.Now().Unix() % 5; modulus {
		case 0:
			sleep = rng.Int63n(2000)
		case 1:
			sleep = rng.Int63n(15)
		case 2:
			sleep = rng.Int63n(917)
		case 3:
			sleep = rng.Int63n(87)
		case 4:
			sleep = rng.Int63n(1173)
		}
		ctx := req.Context()
		span := trace.SpanFromContext(ctx)
		span.SetAttributes(serverAttribute)

		actionChild(tracer, ctx, sleep)

		w.Write([]byte("Hello World"))
	})
	wrappedHandler := otelhttp.NewHandler(handler, "/hello")

	http.Handle("/hello", wrappedHandler)
	http.ListenAndServe(":7080", nil)
}

func initProvider() func() {
	ctx := context.Background()

	otelAgentAddr, xtraceToken, ok := common.ObtainXTraceInfo()

	if !ok {
		log.Fatalf("Cannot init OpenTelemetry, exit")
		os.Exit(-1)
	}

	headers := map[string]string{"Authentication": xtraceToken} // Replace xtraceToken with the authentication token obtained in the Prerequisites section.
	traceClient := otlptracegrpc.NewClient(
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithEndpoint(otelAgentAddr), // Replace otelAgentAddr with the endpoint obtained in the Prerequisites section.
		otlptracegrpc.WithHeaders(headers),
		otlptracegrpc.WithDialOption(grpc.WithBlock()))
	log.Println("start to connect to server")
	traceExp, err := otlptrace.New(ctx, traceClient)
	handleErr(err, "Failed to create the collector trace exporter")

	res, err := resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithProcess(),
		resource.WithTelemetrySDK(),
		resource.WithHost(),
		resource.WithAttributes(
			// Specify the service name displayed on the backend of Tracing Analysis.
			semconv.ServiceNameKey.String(common.ServerServiceName),
		),
	)
	handleErr(err, "failed to create resource")

	bsp := sdktrace.NewBatchSpanProcessor(traceExp)
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)

	// Set the global propagator to tracecontext. The global propagator is not specified by default.
	otel.SetTextMapPropagator(propagation.TraceContext{})
	otel.SetTracerProvider(tracerProvider)

	return func() {
		cxt, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		if err := traceExp.Shutdown(cxt); err != nil {
			otel.Handle(err)
		}
	}
}
*/

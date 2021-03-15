package testdata.v1.util;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 */
@javax.annotation.Generated(
    value = "by gRPC proto compiler (version 1.36.0)",
    comments = "Source: testdata.tld/util/all.proto")
public final class UtilTestsGrpc {

  private UtilTestsGrpc() {}

  public static final String SERVICE_NAME = "testdata.v1.util.UtilTests";

  // Static method descriptors that strictly reflect the proto.
  private static volatile io.grpc.MethodDescriptor<testdata.v1.util.All.UtilTestRequest,
      testdata.v1.util.All.CheckStatusResponse> getUtilTestMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "UtilTest",
      requestType = testdata.v1.util.All.UtilTestRequest.class,
      responseType = testdata.v1.util.All.CheckStatusResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<testdata.v1.util.All.UtilTestRequest,
      testdata.v1.util.All.CheckStatusResponse> getUtilTestMethod() {
    io.grpc.MethodDescriptor<testdata.v1.util.All.UtilTestRequest, testdata.v1.util.All.CheckStatusResponse> getUtilTestMethod;
    if ((getUtilTestMethod = UtilTestsGrpc.getUtilTestMethod) == null) {
      synchronized (UtilTestsGrpc.class) {
        if ((getUtilTestMethod = UtilTestsGrpc.getUtilTestMethod) == null) {
          UtilTestsGrpc.getUtilTestMethod = getUtilTestMethod =
              io.grpc.MethodDescriptor.<testdata.v1.util.All.UtilTestRequest, testdata.v1.util.All.CheckStatusResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "UtilTest"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.lite.ProtoLiteUtils.marshaller(
                  testdata.v1.util.All.UtilTestRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.lite.ProtoLiteUtils.marshaller(
                  testdata.v1.util.All.CheckStatusResponse.getDefaultInstance()))
              .build();
        }
      }
    }
    return getUtilTestMethod;
  }

  /**
   * Creates a new async stub that supports all call types for the service
   */
  public static UtilTestsStub newStub(io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<UtilTestsStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<UtilTestsStub>() {
        @java.lang.Override
        public UtilTestsStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new UtilTestsStub(channel, callOptions);
        }
      };
    return UtilTestsStub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports unary and streaming output calls on the service
   */
  public static UtilTestsBlockingStub newBlockingStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<UtilTestsBlockingStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<UtilTestsBlockingStub>() {
        @java.lang.Override
        public UtilTestsBlockingStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new UtilTestsBlockingStub(channel, callOptions);
        }
      };
    return UtilTestsBlockingStub.newStub(factory, channel);
  }

  /**
   * Creates a new ListenableFuture-style stub that supports unary calls on the service
   */
  public static UtilTestsFutureStub newFutureStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<UtilTestsFutureStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<UtilTestsFutureStub>() {
        @java.lang.Override
        public UtilTestsFutureStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new UtilTestsFutureStub(channel, callOptions);
        }
      };
    return UtilTestsFutureStub.newStub(factory, channel);
  }

  /**
   */
  public static abstract class UtilTestsImplBase implements io.grpc.BindableService {

    /**
     */
    public void utilTest(testdata.v1.util.All.UtilTestRequest request,
        io.grpc.stub.StreamObserver<testdata.v1.util.All.CheckStatusResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getUtilTestMethod(), responseObserver);
    }

    @java.lang.Override public final io.grpc.ServerServiceDefinition bindService() {
      return io.grpc.ServerServiceDefinition.builder(getServiceDescriptor())
          .addMethod(
            getUtilTestMethod(),
            io.grpc.stub.ServerCalls.asyncUnaryCall(
              new MethodHandlers<
                testdata.v1.util.All.UtilTestRequest,
                testdata.v1.util.All.CheckStatusResponse>(
                  this, METHODID_UTIL_TEST)))
          .build();
    }
  }

  /**
   */
  public static final class UtilTestsStub extends io.grpc.stub.AbstractAsyncStub<UtilTestsStub> {
    private UtilTestsStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected UtilTestsStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new UtilTestsStub(channel, callOptions);
    }

    /**
     */
    public void utilTest(testdata.v1.util.All.UtilTestRequest request,
        io.grpc.stub.StreamObserver<testdata.v1.util.All.CheckStatusResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getUtilTestMethod(), getCallOptions()), request, responseObserver);
    }
  }

  /**
   */
  public static final class UtilTestsBlockingStub extends io.grpc.stub.AbstractBlockingStub<UtilTestsBlockingStub> {
    private UtilTestsBlockingStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected UtilTestsBlockingStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new UtilTestsBlockingStub(channel, callOptions);
    }

    /**
     */
    public testdata.v1.util.All.CheckStatusResponse utilTest(testdata.v1.util.All.UtilTestRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getUtilTestMethod(), getCallOptions(), request);
    }
  }

  /**
   */
  public static final class UtilTestsFutureStub extends io.grpc.stub.AbstractFutureStub<UtilTestsFutureStub> {
    private UtilTestsFutureStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected UtilTestsFutureStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new UtilTestsFutureStub(channel, callOptions);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<testdata.v1.util.All.CheckStatusResponse> utilTest(
        testdata.v1.util.All.UtilTestRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getUtilTestMethod(), getCallOptions()), request);
    }
  }

  private static final int METHODID_UTIL_TEST = 0;

  private static final class MethodHandlers<Req, Resp> implements
      io.grpc.stub.ServerCalls.UnaryMethod<Req, Resp>,
      io.grpc.stub.ServerCalls.ServerStreamingMethod<Req, Resp>,
      io.grpc.stub.ServerCalls.ClientStreamingMethod<Req, Resp>,
      io.grpc.stub.ServerCalls.BidiStreamingMethod<Req, Resp> {
    private final UtilTestsImplBase serviceImpl;
    private final int methodId;

    MethodHandlers(UtilTestsImplBase serviceImpl, int methodId) {
      this.serviceImpl = serviceImpl;
      this.methodId = methodId;
    }

    @java.lang.Override
    @java.lang.SuppressWarnings("unchecked")
    public void invoke(Req request, io.grpc.stub.StreamObserver<Resp> responseObserver) {
      switch (methodId) {
        case METHODID_UTIL_TEST:
          serviceImpl.utilTest((testdata.v1.util.All.UtilTestRequest) request,
              (io.grpc.stub.StreamObserver<testdata.v1.util.All.CheckStatusResponse>) responseObserver);
          break;
        default:
          throw new AssertionError();
      }
    }

    @java.lang.Override
    @java.lang.SuppressWarnings("unchecked")
    public io.grpc.stub.StreamObserver<Req> invoke(
        io.grpc.stub.StreamObserver<Resp> responseObserver) {
      switch (methodId) {
        default:
          throw new AssertionError();
      }
    }
  }

  private static volatile io.grpc.ServiceDescriptor serviceDescriptor;

  public static io.grpc.ServiceDescriptor getServiceDescriptor() {
    io.grpc.ServiceDescriptor result = serviceDescriptor;
    if (result == null) {
      synchronized (UtilTestsGrpc.class) {
        result = serviceDescriptor;
        if (result == null) {
          serviceDescriptor = result = io.grpc.ServiceDescriptor.newBuilder(SERVICE_NAME)
              .addMethod(getUtilTestMethod())
              .build();
        }
      }
    }
    return result;
  }
}

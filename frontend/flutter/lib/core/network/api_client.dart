import 'package:dio/dio.dart';
import '../logger/request_logger.dart';

class ApiClient {
  final Dio dio;
  String? _activeTenantId;
  String? _apiKey;

  ApiClient({required String baseUrl})
    : dio = Dio(
        BaseOptions(
          baseUrl: baseUrl,
          connectTimeout: const Duration(seconds: 10),
          receiveTimeout: const Duration(seconds: 10),
          headers: {
            'Content-Type': 'application/json',
            'Accept': 'application/json',
          },
        ),
      ) {
    dio.interceptors.add(
      InterceptorsWrapper(
        onRequest: (options, handler) {
          if (_activeTenantId != null) {
            options.headers['X-Tenant-ID'] = _activeTenantId;
          }
          if (_apiKey != null) {
            options.headers['X-API-Key'] = _apiKey;
          }
          return handler.next(options);
        },
        onError: (DioException error, handler) {
          final requestOptions = error.requestOptions;
          final response = error.response;

          final StringBuffer logBuffer = StringBuffer();
          logBuffer.writeln('=== FAILED REQUEST ===');
          logBuffer.writeln('URI: ${requestOptions.uri}');
          logBuffer.writeln('Method: ${requestOptions.method}');
          logBuffer.writeln('Headers: ${requestOptions.headers}');
          if (requestOptions.data != null) {
            logBuffer.writeln('Request Data: ${requestOptions.data}');
          }
          
          if (response != null) {
            logBuffer.writeln('Status Code: ${response.statusCode}');
            logBuffer.writeln('Response Headers: ${response.headers}');
            logBuffer.writeln('Response Data: ${response.data}');
          } else {
            logBuffer.writeln('No response received (Network timeout/error).');
          }
          logBuffer.writeln('Error Message: ${error.message}');
          logBuffer.writeln('======================');

          RequestLogger.logError(logBuffer.toString());
          return handler.next(error);
        },
      ),
    );
  }

  void updateTenantContext(String tenantId, String? apiKey) {
    _activeTenantId = tenantId;
    _apiKey = apiKey;
  }

  void clearTenantContext() {
    _activeTenantId = null;
    _apiKey = null;
  }
}

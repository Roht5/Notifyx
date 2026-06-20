import 'package:dio/dio.dart';

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
          // Log or map global network errors
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

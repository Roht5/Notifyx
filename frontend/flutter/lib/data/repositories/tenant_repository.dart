import '../../core/network/api_client.dart';
import '../models/tenant_model.dart';

abstract class TenantRepository {
  Future<List<TenantModel>> getTenants();
  Future<TenantModel> createTenant(String name, int globalRateCap);
  Future<TenantModel> updateTenantChannels(String id, List<String> channels);
  Future<TenantModel> updateTenantRateLimits(
    String id,
    Map<String, int> limits,
  );
  Future<String> rotateApiKey(String id);
  Future<void> revokeApiKey(String id, String keyId);
}

class HttpTenantRepository implements TenantRepository {
  final ApiClient apiClient;

  HttpTenantRepository(this.apiClient);

  @override
  Future<List<TenantModel>> getTenants() async {
    final response = await apiClient.dio.get('/api/v1/tenants');
    final List<dynamic> data = response.data ?? [];
    return data
        .map((e) => TenantModel.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  @override
  Future<TenantModel> createTenant(String name, int globalRateCap) async {
    final response = await apiClient.dio.post(
      '/api/v1/tenants',
      data: {'name': name, 'global_rate_cap': globalRateCap},
    );
    return TenantModel.fromJson(response.data as Map<String, dynamic>);
  }

  @override
  Future<TenantModel> updateTenantChannels(
    String id,
    List<String> channels,
  ) async {
    final response = await apiClient.dio.put(
      '/api/v1/tenants/$id/channels',
      data: {'enabled_channels': channels},
    );
    return TenantModel.fromJson(response.data as Map<String, dynamic>);
  }

  @override
  Future<TenantModel> updateTenantRateLimits(
    String id,
    Map<String, int> limits,
  ) async {
    final response = await apiClient.dio.put(
      '/api/v1/tenants/$id/rate-limits',
      data: {'channel_rate_limits': limits},
    );
    return TenantModel.fromJson(response.data as Map<String, dynamic>);
  }

  @override
  Future<String> rotateApiKey(String id) async {
    final response = await apiClient.dio.post('/api/v1/tenants/$id/keys');
    return response.data['api_key'] ?? '';
  }

  @override
  Future<void> revokeApiKey(String id, String keyId) async {
    await apiClient.dio.delete('/api/v1/tenants/$id/keys/$keyId');
  }
}

class MockTenantRepository implements TenantRepository {
  final List<TenantModel> _mockTenants = [
    TenantModel(
      id: 'tenant-acme-uuid',
      name: 'Acme Corp',
      globalRateCap: 500,
      enabledChannels: ['email', 'push', 'websocket'],
      channelRateLimits: {'email': 200, 'push': 250, 'websocket': 100},
      apiKeys: ['ntx_key_acme_live_1a2b3c4d'],
      createdAt: DateTime.now().subtract(const Duration(days: 30)),
    ),
    TenantModel(
      id: 'tenant-globex-uuid',
      name: 'Globex Industries',
      globalRateCap: 1200,
      enabledChannels: ['email', 'sms', 'push'],
      channelRateLimits: {'email': 500, 'sms': 300, 'push': 400},
      apiKeys: ['ntx_key_globex_live_9z8y7x6w'],
      createdAt: DateTime.now().subtract(const Duration(days: 15)),
    ),
  ];

  @override
  Future<List<TenantModel>> getTenants() async {
    await Future.delayed(const Duration(milliseconds: 600));
    return List.from(_mockTenants);
  }

  @override
  Future<TenantModel> createTenant(String name, int globalRateCap) async {
    await Future.delayed(const Duration(milliseconds: 800));
    final newTenant = TenantModel(
      id: 'tenant-${name.toLowerCase().replaceAll(' ', '-')}-uuid',
      name: name,
      globalRateCap: globalRateCap,
      enabledChannels: ['email', 'websocket'],
      channelRateLimits: {'email': 50, 'websocket': 50},
      apiKeys: ['ntx_key_${name.toLowerCase().replaceAll(' ', '_')}_temp_key'],
      createdAt: DateTime.now(),
    );
    _mockTenants.add(newTenant);
    return newTenant;
  }

  @override
  Future<TenantModel> updateTenantChannels(
    String id,
    List<String> channels,
  ) async {
    await Future.delayed(const Duration(milliseconds: 500));
    final index = _mockTenants.indexWhere((t) => t.id == id);
    if (index != -1) {
      final updated = _mockTenants[index].copyWith(enabledChannels: channels);
      _mockTenants[index] = updated;
      return updated;
    }
    throw Exception('Tenant not found');
  }

  @override
  Future<TenantModel> updateTenantRateLimits(
    String id,
    Map<String, int> limits,
  ) async {
    await Future.delayed(const Duration(milliseconds: 500));
    final index = _mockTenants.indexWhere((t) => t.id == id);
    if (index != -1) {
      final updated = _mockTenants[index].copyWith(channelRateLimits: limits);
      _mockTenants[index] = updated;
      return updated;
    }
    throw Exception('Tenant not found');
  }

  @override
  Future<String> rotateApiKey(String id) async {
    await Future.delayed(const Duration(milliseconds: 600));
    final index = _mockTenants.indexWhere((t) => t.id == id);
    if (index != -1) {
      final newKey = 'ntx_key_rotated_${DateTime.now().millisecondsSinceEpoch}';
      final updatedKeys = List<String>.from(_mockTenants[index].apiKeys)
        ..add(newKey);
      _mockTenants[index] = _mockTenants[index].copyWith(apiKeys: updatedKeys);
      return newKey;
    }
    throw Exception('Tenant not found');
  }

  @override
  Future<void> revokeApiKey(String id, String keyId) async {
    await Future.delayed(const Duration(milliseconds: 400));
    final index = _mockTenants.indexWhere((t) => t.id == id);
    if (index != -1) {
      final updatedKeys = List<String>.from(_mockTenants[index].apiKeys)
        ..remove(keyId);
      _mockTenants[index] = _mockTenants[index].copyWith(apiKeys: updatedKeys);
      return;
    }
    throw Exception('Tenant not found');
  }
}

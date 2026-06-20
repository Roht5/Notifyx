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
    
    final List<TenantModel> tenants = [];
    for (final item in data) {
      final tenantId = item['id'].toString();
      
      // Fetch key metadata associated with this tenant
      List<String> apiKeysList = [];
      try {
        final keysResponse = await apiClient.dio.get('/api/v1/tenants/$tenantId/keys');
        final List<dynamic> keysData = keysResponse.data ?? [];
        apiKeysList = keysData.map((e) => 'nfx_key_metadata_${e['id']}').toList();
      } catch (e) {
        // Fallback if not configured
      }

      // Fetch actual channels configured
      List<String> enabledChannels = [];
      try {
        final channelsResponse = await apiClient.dio.get('/api/v1/tenants/$tenantId/channels');
        final List<dynamic> channelsData = channelsResponse.data ?? [];
        for (final ch in channelsData) {
          if (ch['enabled'] == true) {
            final backendChannel = ch['channel'].toString();
            enabledChannels.add(backendChannel == 'inapp' ? 'websocket' : backendChannel);
          }
        }
      } catch (e) {
        // Fallback to all enabled if not configured in DB yet
        enabledChannels = <String>['email', 'push', 'sms', 'websocket'];
      }

      // Fetch actual rate limits configured
      final Map<String, int> rateLimits = {};
      int globalCap = item['global_rate_cap'] ?? 300;
      try {
        final limitsResponse = await apiClient.dio.get('/api/v1/tenants/$tenantId/rate-limits');
        final limitsData = limitsResponse.data as Map<String, dynamic>;
        globalCap = limitsData['global_cap'] ?? globalCap;
        final List<dynamic> limitsList = limitsData['rate_limits'] ?? [];
        for (final rl in limitsList) {
          final backendChannel = rl['channel'].toString();
          final frontendChannel = backendChannel == 'inapp' ? 'websocket' : backendChannel;
          rateLimits[frontendChannel] = rl['max_per_min'] as int;
        }
      } catch (e) {
        // Set default limits
        for (final ch in enabledChannels) {
          rateLimits[ch] = 60;
        }
      }
      
      tenants.add(TenantModel.fromJson({
        ...item as Map<String, dynamic>,
        'global_rate_cap': globalCap,
        'api_keys': apiKeysList,
        'enabled_channels': enabledChannels,
        'channel_rate_limits': rateLimits,
      }));
    }
    return tenants;
  }

  @override
  Future<TenantModel> createTenant(String name, int globalRateCap) async {
    // Backend POST /api/v1/tenants only accepts {"name": string}
    final response = await apiClient.dio.post(
      '/api/v1/tenants',
      data: {'name': name},
    );
    final responseData = response.data as Map<String, dynamic>;
    final tenantId = responseData['id'] ?? '';
    final rawApiKey = responseData['api_key'] ?? '';

    // Seed the global cap immediately after creation via PUT /api/v1/tenants/:id/rate-limits
    int currentGlobalRateCap = globalRateCap;
    try {
      final rateLimitResponse = await apiClient.dio.put(
        '/api/v1/tenants/$tenantId/rate-limits',
        data: {
          'global_cap': globalRateCap,
          'rate_limits': [],
        },
      );
      final rateLimitData = rateLimitResponse.data as Map<String, dynamic>;
      currentGlobalRateCap = rateLimitData['global_cap'] ?? globalRateCap;
    } catch (e) {
      // ignore
    }

    return TenantModel.fromJson({
      ...responseData,
      'global_rate_cap': currentGlobalRateCap,
      'api_keys': [rawApiKey],
      'enabled_channels': <String>['email', 'push', 'sms', 'websocket'],
      'channel_rate_limits': <String, int>{
        'email': 60,
        'push': 60,
        'sms': 60,
        'websocket': 60,
      },
    });
  }

  @override
  Future<TenantModel> updateTenantChannels(
    String id,
    List<String> channels,
  ) async {
    // Map list to backend channel object models: {"channels": [{"channel": string, "enabled": bool}]}
    final List<Map<String, dynamic>> channelList = [];
    final allChannels = ['email', 'push', 'sms', 'inapp'];
    for (final ch in allChannels) {
      final mappedCh = ch == 'inapp' ? 'websocket' : ch;
      channelList.add({
        'channel': ch,
        'enabled': channels.contains(mappedCh),
      });
    }

    final response = await apiClient.dio.put(
      '/api/v1/tenants/$id/channels',
      data: {'channels': channelList},
    );
    
    final List<dynamic> data = response.data ?? [];
    final List<String> enabledChannels = [];
    for (final item in data) {
      if (item['enabled'] == true) {
        final backendChannel = item['channel'].toString();
        enabledChannels.add(backendChannel == 'inapp' ? 'websocket' : backendChannel);
      }
    }

    // Fetch base tenant details
    final tenantResponse = await apiClient.dio.get('/api/v1/tenants/$id');
    final tenantJson = tenantResponse.data as Map<String, dynamic>;
    
    // Fetch key list
    List<String> apiKeysList = [];
    try {
      final keysResponse = await apiClient.dio.get('/api/v1/tenants/$id/keys');
      final List<dynamic> keysData = keysResponse.data ?? [];
      apiKeysList = keysData.map((e) => 'nfx_key_metadata_${e['id']}').toList();
    } catch (e) {
      // ignore
    }

    // Fetch actual rate limits configured
    final Map<String, int> rateLimits = {};
    try {
      final limitsResponse = await apiClient.dio.get('/api/v1/tenants/$id/rate-limits');
      final limitsData = limitsResponse.data as Map<String, dynamic>;
      final List<dynamic> limitsList = limitsData['rate_limits'] ?? [];
      for (final rl in limitsList) {
        final backendChannel = rl['channel'].toString();
        final frontendChannel = backendChannel == 'inapp' ? 'websocket' : backendChannel;
        rateLimits[frontendChannel] = rl['max_per_min'] as int;
      }
    } catch (e) {
      // ignore
    }

    return TenantModel.fromJson({
      ...tenantJson,
      'api_keys': apiKeysList,
      'enabled_channels': enabledChannels,
      'channel_rate_limits': rateLimits,
    });
  }

  @override
  Future<TenantModel> updateTenantRateLimits(
    String id,
    Map<String, int> limits,
  ) async {
    // Map limits map to backend object model: {"rate_limits": [{"channel": string, "max_per_min": int}]}
    final List<Map<String, dynamic>> rateLimits = [];
    limits.forEach((channel, maxPerMin) {
      final backendChannel = channel == 'websocket' ? 'inapp' : channel;
      rateLimits.add({
        'channel': backendChannel,
        'max_per_min': maxPerMin,
      });
    });

    final response = await apiClient.dio.put(
      '/api/v1/tenants/$id/rate-limits',
      data: {
        'rate_limits': rateLimits,
      },
    );
    
    final responseData = response.data as Map<String, dynamic>;
    final int globalRateCap = responseData['global_cap'] ?? 300;
    final List<dynamic> limitsList = responseData['rate_limits'] ?? [];
    
    final Map<String, int> parsedLimits = {};
    for (final item in limitsList) {
      final backendChannel = item['channel'].toString();
      final frontendChannel = backendChannel == 'inapp' ? 'websocket' : backendChannel;
      parsedLimits[frontendChannel] = item['max_per_min'] as int;
    }

    // Fetch base tenant details
    final tenantResponse = await apiClient.dio.get('/api/v1/tenants/$id');
    final tenantJson = tenantResponse.data as Map<String, dynamic>;

    // Fetch keys list
    List<String> apiKeysList = [];
    try {
      final keysResponse = await apiClient.dio.get('/api/v1/tenants/$id/keys');
      final List<dynamic> keysData = keysResponse.data ?? [];
      apiKeysList = keysData.map((e) => 'nfx_key_metadata_${e['id']}').toList();
    } catch (e) {
      // ignore
    }

    // Fetch actual channels configured
    final List<String> enabledChannels = [];
    try {
      final channelsResponse = await apiClient.dio.get('/api/v1/tenants/$id/channels');
      final List<dynamic> channelsData = channelsResponse.data ?? [];
      for (final ch in channelsData) {
        if (ch['enabled'] == true) {
          final backendChannel = ch['channel'].toString();
          enabledChannels.add(backendChannel == 'inapp' ? 'websocket' : backendChannel);
        }
      }
    } catch (e) {
      // ignore
    }

    return TenantModel.fromJson({
      ...tenantJson,
      'global_rate_cap': globalRateCap,
      'api_keys': apiKeysList,
      'enabled_channels': enabledChannels.isEmpty ? parsedLimits.keys.toList() : enabledChannels,
      'channel_rate_limits': parsedLimits,
    });
  }

  @override
  Future<String> rotateApiKey(String id) async {
    final response = await apiClient.dio.post('/api/v1/tenants/$id/keys');
    return response.data['api_key'] ?? '';
  }

  @override
  Future<void> revokeApiKey(String id, String keyId) async {
    // keyId is nfx_key_metadata_<UUID>, extract UUID suffix if prepended
    final actualKeyId = keyId.startsWith('nfx_key_metadata_') 
        ? keyId.substring('nfx_key_metadata_'.length) 
        : keyId;
    await apiClient.dio.delete('/api/v1/tenants/$id/keys/$actualKeyId');
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

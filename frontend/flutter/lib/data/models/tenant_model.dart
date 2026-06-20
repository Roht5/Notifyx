class TenantModel {
  final String id;
  final String name;
  final int globalRateCap;
  final List<String> enabledChannels;
  final Map<String, int> channelRateLimits;
  final List<String> apiKeys;
  final DateTime createdAt;

  TenantModel({
    required this.id,
    required this.name,
    required this.globalRateCap,
    required this.enabledChannels,
    required this.channelRateLimits,
    required this.apiKeys,
    required this.createdAt,
  });

  factory TenantModel.fromJson(Map<String, dynamic> json) {
    // Parse channels enabled lists
    final List<dynamic> channelsJson = json['enabled_channels'] ?? [];
    final List<String> channels = channelsJson
        .map((e) => e.toString())
        .toList();

    // Parse rate limits maps
    final Map<String, dynamic> limitsJson = json['channel_rate_limits'] ?? {};
    final Map<String, int> limits = limitsJson.map(
      (k, v) => MapEntry(k, v as int),
    );

    // Parse api keys lists
    final List<dynamic> keysJson = json['api_keys'] ?? [];
    final List<String> keys = keysJson.map((e) => e.toString()).toList();

    return TenantModel(
      id: json['id'] ?? '',
      name: json['name'] ?? '',
      globalRateCap: json['global_rate_cap'] ?? 0,
      enabledChannels: channels,
      channelRateLimits: limits,
      apiKeys: keys,
      createdAt: json['created_at'] != null
          ? DateTime.parse(json['created_at'])
          : DateTime.now(),
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'name': name,
      'global_rate_cap': globalRateCap,
      'enabled_channels': enabledChannels,
      'channel_rate_limits': channelRateLimits,
      'api_keys': apiKeys,
      'created_at': createdAt.toIso8601String(),
    };
  }

  TenantModel copyWith({
    String? id,
    String? name,
    int? globalRateCap,
    List<String>? enabledChannels,
    Map<String, int>? channelRateLimits,
    List<String>? apiKeys,
    DateTime? createdAt,
  }) {
    return TenantModel(
      id: id ?? this.id,
      name: name ?? this.name,
      globalRateCap: globalRateCap ?? this.globalRateCap,
      enabledChannels: enabledChannels ?? this.enabledChannels,
      channelRateLimits: channelRateLimits ?? this.channelRateLimits,
      apiKeys: apiKeys ?? this.apiKeys,
      createdAt: createdAt ?? this.createdAt,
    );
  }
}

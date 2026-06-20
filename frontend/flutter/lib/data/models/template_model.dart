class TemplateModel {
  final String id;
  final String tenantId;
  final String name;
  final String channel;
  final String?
  subject; // Nullable because push/sms/inapp might not have subjects
  final String body;
  final DateTime createdAt;

  TemplateModel({
    required this.id,
    required this.tenantId,
    required this.name,
    required this.channel,
    this.subject,
    required this.body,
    required this.createdAt,
  });

  factory TemplateModel.fromJson(Map<String, dynamic> json) {
    return TemplateModel(
      id: json['id'] ?? '',
      tenantId: json['tenant_id'] ?? '',
      name: json['name'] ?? '',
      channel: json['channel'] == 'inapp' ? 'websocket' : (json['channel'] ?? ''),
      subject: json['subject'],
      body: json['body'] ?? '',
      createdAt: json['created_at'] != null
          ? DateTime.parse(json['created_at'])
          : DateTime.now(),
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'tenant_id': tenantId,
      'name': name,
      'channel': channel == 'websocket' ? 'inapp' : channel,
      'subject': subject,
      'body': body,
      'created_at': createdAt.toIso8601String(),
    };
  }

  TemplateModel copyWith({
    String? id,
    String? tenantId,
    String? name,
    String? channel,
    String? subject,
    String? body,
    DateTime? createdAt,
  }) {
    return TemplateModel(
      id: id ?? this.id,
      tenantId: tenantId ?? this.tenantId,
      name: name ?? this.name,
      channel: channel ?? this.channel,
      subject: subject ?? this.subject,
      body: body ?? this.body,
      createdAt: createdAt ?? this.createdAt,
    );
  }
}

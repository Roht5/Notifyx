class NotificationModel {
  final String id;
  final String tenantId;
  final String channel;
  final String priority;
  final String status;
  final String recipientId;
  final String? recipientEmail;
  final String? recipientPhone;
  final String? recipientToken;
  final String? templateId;
  final String? subject;
  final String body;
  final Map<String, dynamic> metadata;
  final String? idempotencyKey;
  final DateTime? scheduledAt;
  final DateTime createdAt;
  final List<DeliveryAttemptModel> attempts;

  NotificationModel({
    required this.id,
    required this.tenantId,
    required this.channel,
    required this.priority,
    required this.status,
    required this.recipientId,
    this.recipientEmail,
    this.recipientPhone,
    this.recipientToken,
    this.templateId,
    this.subject,
    required this.body,
    required this.metadata,
    this.idempotencyKey,
    this.scheduledAt,
    required this.createdAt,
    required this.attempts,
  });

  factory NotificationModel.fromJson(Map<String, dynamic> json) {
    final List<dynamic> attemptsJson = json['attempts'] ?? [];
    final List<DeliveryAttemptModel> attemptsList = attemptsJson
        .map((e) => DeliveryAttemptModel.fromJson(e as Map<String, dynamic>))
        .toList();

    return NotificationModel(
      id: json['id'] ?? '',
      tenantId: json['tenant_id'] ?? '',
      channel: json['channel'] ?? '',
      priority: json['priority'] ?? 'normal',
      status: json['status'] ?? 'pending',
      recipientId: json['recipient_id'] ?? '',
      recipientEmail: json['recipient_email'],
      recipientPhone: json['recipient_phone'],
      recipientToken: json['recipient_token'],
      templateId: json['template_id'],
      subject: json['subject'],
      body: json['body'] ?? '',
      metadata: json['metadata'] ?? {},
      idempotencyKey: json['idempotency_key'],
      scheduledAt: json['scheduled_at'] != null
          ? DateTime.parse(json['scheduled_at'])
          : null,
      createdAt: json['created_at'] != null
          ? DateTime.parse(json['created_at'])
          : DateTime.now(),
      attempts: attemptsList,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'tenant_id': tenantId,
      'channel': channel,
      'priority': priority,
      'status': status,
      'recipient_id': recipientId,
      'recipient_email': recipientEmail,
      'recipient_phone': recipientPhone,
      'recipient_token': recipientToken,
      'template_id': templateId,
      'subject': subject,
      'body': body,
      'metadata': metadata,
      'idempotency_key': idempotencyKey,
      'scheduled_at': scheduledAt?.toIso8601String(),
      'created_at': createdAt.toIso8601String(),
      'attempts': attempts.map((e) => e.toJson()).toList(),
    };
  }
}

class DeliveryAttemptModel {
  final String id;
  final String status;
  final int attemptNumber;
  final String? errorMessage;
  final DateTime? deliveredAt;
  final DateTime createdAt;

  DeliveryAttemptModel({
    required this.id,
    required this.status,
    required this.attemptNumber,
    this.errorMessage,
    this.deliveredAt,
    required this.createdAt,
  });

  factory DeliveryAttemptModel.fromJson(Map<String, dynamic> json) {
    return DeliveryAttemptModel(
      id: json['id'] ?? '',
      status: json['status'] ?? '',
      attemptNumber: json['attempts'] ?? 1,
      errorMessage: json['error_message'],
      deliveredAt: json['delivered_at'] != null
          ? DateTime.parse(json['delivered_at'])
          : null,
      createdAt: json['created_at'] != null
          ? DateTime.parse(json['created_at'])
          : DateTime.now(),
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'status': status,
      'attempts': attemptNumber,
      'error_message': errorMessage,
      'delivered_at': deliveredAt?.toIso8601String(),
      'created_at': createdAt.toIso8601String(),
    };
  }
}

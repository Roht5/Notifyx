import 'package:equatable/equatable.dart';

abstract class NotificationEvent extends Equatable {
  const NotificationEvent();

  @override
  List<Object?> get props => [];
}

class LoadNotificationHistory extends NotificationEvent {
  final String tenantId;
  final String? status;
  final String? channel;
  final String? recipientSearch;
  final bool refresh;

  const LoadNotificationHistory({
    required this.tenantId,
    this.status,
    this.channel,
    this.recipientSearch,
    this.refresh = false,
  });

  @override
  List<Object?> get props => [
    tenantId,
    status,
    channel,
    recipientSearch,
    refresh,
  ];
}

class SendNotificationEvent extends NotificationEvent {
  final String tenantId;
  final String channel;
  final String recipientId;
  final String? recipientEmail;
  final String? recipientPhone;
  final String? recipientToken;
  final String priority;
  final String? templateId;
  final String? subject;
  final String body;
  final Map<String, dynamic> metadata;
  final String? idempotencyKey;

  const SendNotificationEvent({
    required this.tenantId,
    required this.channel,
    required this.recipientId,
    this.recipientEmail,
    this.recipientPhone,
    this.recipientToken,
    required this.priority,
    this.templateId,
    this.subject,
    required this.body,
    required this.metadata,
    this.idempotencyKey,
  });

  @override
  List<Object?> get props => [
    tenantId,
    channel,
    recipientId,
    recipientEmail,
    recipientPhone,
    recipientToken,
    priority,
    templateId,
    subject,
    body,
    metadata,
    idempotencyKey,
  ];
}

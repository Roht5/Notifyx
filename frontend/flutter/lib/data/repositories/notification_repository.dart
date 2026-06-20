import '../../core/network/api_client.dart';
import '../models/notification_model.dart';

abstract class NotificationRepository {
  Future<List<NotificationModel>> getHistory({
    required String tenantId,
    String? status,
    String? channel,
    String? recipientSearch,
    int limit = 50,
    int offset = 0,
  });
  Future<NotificationModel> sendNotification({
    required String tenantId,
    required String channel,
    required String recipientId,
    String? recipientEmail,
    String? recipientPhone,
    String? recipientToken,
    required String priority,
    String? templateId,
    String? subject,
    required String body,
    required Map<String, dynamic> metadata,
    String? idempotencyKey,
  });
}

class HttpNotificationRepository implements NotificationRepository {
  final ApiClient apiClient;

  HttpNotificationRepository(this.apiClient);

  @override
  Future<List<NotificationModel>> getHistory({
    required String tenantId,
    String? status,
    String? channel,
    String? recipientSearch,
    int limit = 50,
    int offset = 0,
  }) async {
    final Map<String, dynamic> queryParams = {'limit': limit, 'offset': offset};
    if (status != null && status.isNotEmpty) queryParams['status'] = status;
    if (channel != null && channel.isNotEmpty) queryParams['channel'] = channel;
    if (recipientSearch != null && recipientSearch.isNotEmpty) {
      queryParams['recipient'] = recipientSearch;
    }

    final response = await apiClient.dio.get(
      '/api/v1/notifications/history',
      queryParameters: queryParams,
    );
    final Map<String, dynamic> body = response.data ?? {};
    final List<dynamic> data = body['notifications'] ?? [];
    return data
        .map((e) => NotificationModel.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  @override
  Future<NotificationModel> sendNotification({
    required String tenantId,
    required String channel,
    required String recipientId,
    String? recipientEmail,
    String? recipientPhone,
    String? recipientToken,
    required String priority,
    String? templateId,
    String? subject,
    required String body,
    required Map<String, dynamic> metadata,
    String? idempotencyKey,
  }) async {
    final Map<String, dynamic> postData = {
      'channel': channel,
      'recipient_id': recipientId,
      'priority': priority,
      'body': body,
      'metadata': metadata,
    };
    if (recipientEmail != null) postData['recipient_email'] = recipientEmail;
    if (recipientPhone != null) postData['recipient_phone'] = recipientPhone;
    if (recipientToken != null) postData['recipient_token'] = recipientToken;
    if (templateId != null) postData['template_id'] = templateId;
    if (subject != null) postData['subject'] = subject;
    if (idempotencyKey != null) postData['idempotency_key'] = idempotencyKey;

    final response = await apiClient.dio.post(
      '/api/v1/notifications/send',
      data: postData,
    );
    return NotificationModel.fromJson(response.data as Map<String, dynamic>);
  }
}

class MockNotificationRepository implements NotificationRepository {
  final List<NotificationModel> _mockNotifications = [
    NotificationModel(
      id: 'notif-1001-uuid',
      tenantId: 'tenant-acme-uuid',
      channel: 'email',
      priority: 'high',
      status: 'delivered',
      recipientId: 'user-001',
      recipientEmail: 'jane.doe@example.com',
      body: 'Hi Jane, your Acme account has been verified successfully!',
      metadata: {'action': 'verification_completed'},
      createdAt: DateTime.now().subtract(const Duration(hours: 2)),
      attempts: [
        DeliveryAttemptModel(
          id: 'att-1a-uuid',
          status: 'delivered',
          attemptNumber: 1,
          deliveredAt: DateTime.now().subtract(const Duration(hours: 2)),
          createdAt: DateTime.now().subtract(
            const Duration(hours: 2, minutes: 1),
          ),
        ),
      ],
    ),
    NotificationModel(
      id: 'notif-1002-uuid',
      tenantId: 'tenant-acme-uuid',
      channel: 'sms',
      priority: 'critical',
      status: 'failed',
      recipientId: 'user-002',
      recipientPhone: '+919876543210',
      body: 'OTP code 4892 is active for the next 5 minutes.',
      metadata: {'purpose': '2fa_login'},
      createdAt: DateTime.now().subtract(const Duration(minutes: 45)),
      attempts: [
        DeliveryAttemptModel(
          id: 'att-2a-uuid',
          status: 'failed',
          attemptNumber: 1,
          errorMessage: 'Fast2SMS API Response Throttled',
          createdAt: DateTime.now().subtract(const Duration(minutes: 44)),
        ),
        DeliveryAttemptModel(
          id: 'att-2b-uuid',
          status: 'failed',
          attemptNumber: 2,
          errorMessage: 'Fast2SMS Provider Rate Limit Reached',
          createdAt: DateTime.now().subtract(const Duration(minutes: 42)),
        ),
      ],
    ),
    NotificationModel(
      id: 'notif-1003-uuid',
      tenantId: 'tenant-globex-uuid',
      channel: 'push',
      priority: 'normal',
      status: 'delivered',
      recipientId: 'user-999',
      recipientToken: 'fcm_device_token_xyz_123',
      body:
          'Globex News: Weekly stock updates are now available in your portfolio.',
      metadata: {'category': 'newsletter'},
      createdAt: DateTime.now().subtract(const Duration(hours: 8)),
      attempts: [
        DeliveryAttemptModel(
          id: 'att-3a-uuid',
          status: 'delivered',
          attemptNumber: 1,
          deliveredAt: DateTime.now().subtract(
            const Duration(hours: 7, minutes: 59),
          ),
          createdAt: DateTime.now().subtract(const Duration(hours: 8)),
        ),
      ],
    ),
  ];

  @override
  Future<List<NotificationModel>> getHistory({
    required String tenantId,
    String? status,
    String? channel,
    String? recipientSearch,
    int limit = 50,
    int offset = 0,
  }) async {
    await Future.delayed(const Duration(milliseconds: 600));

    Iterable<NotificationModel> filtered = _mockNotifications.where(
      (n) => n.tenantId == tenantId,
    );

    if (status != null && status.isNotEmpty) {
      filtered = filtered.where(
        (n) => n.status.toLowerCase() == status.toLowerCase(),
      );
    }
    if (channel != null && channel.isNotEmpty) {
      filtered = filtered.where(
        (n) => n.channel.toLowerCase() == channel.toLowerCase(),
      );
    }
    if (recipientSearch != null && recipientSearch.isNotEmpty) {
      filtered = filtered.where(
        (n) =>
            n.recipientId.contains(recipientSearch) ||
            (n.recipientEmail != null &&
                n.recipientEmail!.contains(recipientSearch)) ||
            (n.recipientPhone != null &&
                n.recipientPhone!.contains(recipientSearch)),
      );
    }

    // Sort by createdAt descending
    final list = filtered.toList()
      ..sort((a, b) => b.createdAt.compareTo(a.createdAt));

    final int start = offset;
    if (start >= list.length) return [];
    final int end = (start + limit) > list.length
        ? list.length
        : (start + limit);

    return list.sublist(start, end);
  }

  @override
  Future<NotificationModel> sendNotification({
    required String tenantId,
    required String channel,
    required String recipientId,
    String? recipientEmail,
    String? recipientPhone,
    String? recipientToken,
    required String priority,
    String? templateId,
    String? subject,
    required String body,
    required Map<String, dynamic> metadata,
    String? idempotencyKey,
  }) async {
    await Future.delayed(const Duration(milliseconds: 800));

    final newNotif = NotificationModel(
      id: 'notif-${DateTime.now().millisecondsSinceEpoch}-uuid',
      tenantId: tenantId,
      channel: channel,
      priority: priority,
      status: 'queued', // Backend inserts as queued/pending
      recipientId: recipientId,
      recipientEmail: recipientEmail,
      recipientPhone: recipientPhone,
      recipientToken: recipientToken,
      templateId: templateId,
      subject: subject,
      body: body,
      metadata: metadata,
      idempotencyKey: idempotencyKey,
      createdAt: DateTime.now(),
      attempts: [],
    );

    _mockNotifications.add(newNotif);

    // Simulate async delivery in Mock Mode (adds an attempt log soon after)
    Future.delayed(const Duration(seconds: 4), () {
      final idx = _mockNotifications.indexWhere(
        (element) => element.id == newNotif.id,
      );
      if (idx != -1) {
        final current = _mockNotifications[idx];
        _mockNotifications[idx] = NotificationModel(
          id: current.id,
          tenantId: current.tenantId,
          channel: current.channel,
          priority: current.priority,
          status: 'delivered',
          recipientId: current.recipientId,
          recipientEmail: current.recipientEmail,
          recipientPhone: current.recipientPhone,
          recipientToken: current.recipientToken,
          templateId: current.templateId,
          subject: current.subject,
          body: current.body,
          metadata: current.metadata,
          idempotencyKey: current.idempotencyKey,
          createdAt: current.createdAt,
          attempts: [
            DeliveryAttemptModel(
              id: 'att-${DateTime.now().millisecondsSinceEpoch}-uuid',
              status: 'delivered',
              attemptNumber: 1,
              deliveredAt: DateTime.now(),
              createdAt: DateTime.now().subtract(const Duration(seconds: 1)),
            ),
          ],
        );
      }
    });

    return newNotif;
  }
}

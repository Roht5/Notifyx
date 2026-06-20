import '../../core/network/api_client.dart';
import '../models/analytics_model.dart';

abstract class AnalyticsRepository {
  Future<AnalyticsModel> getAnalytics(String tenantId);
}

class HttpAnalyticsRepository implements AnalyticsRepository {
  final ApiClient apiClient;

  HttpAnalyticsRepository(this.apiClient);

  @override
  Future<AnalyticsModel> getAnalytics(String tenantId) async {
    final response = await apiClient.dio.get(
      '/api/v1/tenants/$tenantId/analytics',
    );
    return AnalyticsModel.fromJson(response.data as Map<String, dynamic>);
  }
}

class MockAnalyticsRepository implements AnalyticsRepository {
  @override
  Future<AnalyticsModel> getAnalytics(String tenantId) async {
    await Future.delayed(const Duration(milliseconds: 700));

    // Generate mock DLQ trend data (last 7 days)
    final List<DlqTrendPoint> dlqPoints = List.generate(7, (index) {
      final date = DateTime.now().subtract(Duration(days: 6 - index));
      // Random failures count (simulate a spike on day 3)
      int count = 2 + (index % 3) * 3;
      if (index == 4) count = 25; // Spike!
      return DlqTrendPoint(date: date, count: count);
    });

    // Return mock analytics
    return AnalyticsModel(
      totalSent: 1542,
      totalDelivered: 1512,
      totalFailed: 30,
      channels: [
        ChannelBreakdown(
          channel: 'email',
          sent: 720,
          delivered: 714,
          failed: 6,
          deliveryRate: 99.2,
        ),
        ChannelBreakdown(
          channel: 'push',
          sent: 510,
          delivered: 497,
          failed: 13,
          deliveryRate: 97.4,
        ),
        ChannelBreakdown(
          channel: 'sms',
          sent: 212,
          delivered: 194,
          failed: 18,
          deliveryRate: 91.5,
        ),
        ChannelBreakdown(
          channel: 'websocket',
          sent: 100,
          delivered: 100,
          failed: 0,
          deliveryRate: 100.0,
        ),
      ],
      dlqTrend: dlqPoints,
    );
  }
}

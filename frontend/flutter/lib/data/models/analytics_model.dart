class AnalyticsModel {
  final int totalSent;
  final int totalDelivered;
  final int totalFailed;
  final List<ChannelBreakdown> channels;
  final List<DlqTrendPoint> dlqTrend;

  AnalyticsModel({
    required this.totalSent,
    required this.totalDelivered,
    required this.totalFailed,
    required this.channels,
    required this.dlqTrend,
  });

  double get overallDeliveryRate =>
      totalSent == 0 ? 0.0 : (totalDelivered / totalSent) * 100;

  factory AnalyticsModel.fromJson(Map<String, dynamic> json) {
    final Map<String, dynamic> summary = json['summary'] ?? {};

    final List<dynamic> channelsJson = json['channels'] ?? [];
    final List<ChannelBreakdown> channels = channelsJson
        .map((e) => ChannelBreakdown.fromJson(e as Map<String, dynamic>))
        .toList();

    final List<dynamic> dlqList = json['dlq_trend'] ?? [];
    final List<DlqTrendPoint> dlqTrendPoints = dlqList
        .map((e) => DlqTrendPoint.fromJson(e as Map<String, dynamic>))
        .toList();

    return AnalyticsModel(
      totalSent: summary['total_sent'] ?? 0,
      totalDelivered: summary['total_delivered'] ?? 0,
      totalFailed: summary['total_failed'] ?? 0,
      channels: channels,
      dlqTrend: dlqTrendPoints,
    );
  }
}

class ChannelBreakdown {
  final String channel;
  final int sent;
  final int delivered;
  final int failed;
  final double deliveryRate;

  ChannelBreakdown({
    required this.channel,
    required this.sent,
    required this.delivered,
    required this.failed,
    required this.deliveryRate,
  });

  factory ChannelBreakdown.fromJson(Map<String, dynamic> json) {
    return ChannelBreakdown(
      channel: json['channel'] ?? '',
      sent: json['sent'] ?? 0,
      delivered: json['delivered'] ?? 0,
      failed: json['failed'] ?? 0,
      deliveryRate: (json['delivery_rate'] ?? 0.0).toDouble(),
    );
  }
}

class DlqTrendPoint {
  final DateTime date;
  final int count;

  DlqTrendPoint({required this.date, required this.count});

  factory DlqTrendPoint.fromJson(Map<String, dynamic> json) {
    return DlqTrendPoint(
      date: json['date'] != null
          ? DateTime.parse(json['date'])
          : DateTime.now(),
      count: json['count'] ?? 0,
    );
  }
}

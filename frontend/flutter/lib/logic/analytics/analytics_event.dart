import 'package:equatable/equatable.dart';

abstract class AnalyticsEvent extends Equatable {
  const AnalyticsEvent();

  @override
  List<Object?> get props => [];
}

class LoadAnalytics extends AnalyticsEvent {
  final String tenantId;

  const LoadAnalytics(this.tenantId);

  @override
  List<Object?> get props => [tenantId];
}

import 'package:equatable/equatable.dart';
import '../../data/models/analytics_model.dart';

abstract class AnalyticsState extends Equatable {
  const AnalyticsState();

  @override
  List<Object?> get props => [];
}

class AnalyticsInitial extends AnalyticsState {}

class AnalyticsLoading extends AnalyticsState {}

class AnalyticsLoaded extends AnalyticsState {
  final AnalyticsModel analytics;

  const AnalyticsLoaded(this.analytics);

  @override
  List<Object?> get props => [analytics];
}

class AnalyticsFailure extends AnalyticsState {
  final String error;

  const AnalyticsFailure(this.error);

  @override
  List<Object?> get props => [error];
}

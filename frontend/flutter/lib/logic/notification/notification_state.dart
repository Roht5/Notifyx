import 'package:equatable/equatable.dart';
import '../../data/models/notification_model.dart';

abstract class NotificationState extends Equatable {
  const NotificationState();

  @override
  List<Object?> get props => [];
}

class NotificationInitial extends NotificationState {}

class NotificationHistoryLoading extends NotificationState {}

class NotificationHistoryLoaded extends NotificationState {
  final List<NotificationModel> history;
  final bool hasReachedMax;

  const NotificationHistoryLoaded({
    required this.history,
    required this.hasReachedMax,
  });

  @override
  List<Object?> get props => [history, hasReachedMax];
}

class NotificationHistoryFailure extends NotificationState {
  final String error;

  const NotificationHistoryFailure(this.error);

  @override
  List<Object?> get props => [error];
}

class NotificationSendProgress extends NotificationState {}

class NotificationSendSuccess extends NotificationState {
  final NotificationModel notification;

  const NotificationSendSuccess(this.notification);

  @override
  List<Object?> get props => [notification];
}

class NotificationSendFailure extends NotificationState {
  final String error;

  const NotificationSendFailure(this.error);

  @override
  List<Object?> get props => [error];
}

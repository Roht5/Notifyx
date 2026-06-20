import 'package:flutter_bloc/flutter_bloc.dart';
import '../../data/repositories/notification_repository.dart';
import '../../data/models/notification_model.dart';
import 'notification_event.dart';
import 'notification_state.dart';

class NotificationBloc extends Bloc<NotificationEvent, NotificationState> {
  final NotificationRepository notificationRepository;

  NotificationBloc({required this.notificationRepository})
    : super(NotificationInitial()) {
    on<LoadNotificationHistory>(_onLoadHistory);
    on<SendNotificationEvent>(_onSendNotification);
  }

  Future<void> _onLoadHistory(
    LoadNotificationHistory event,
    Emitter<NotificationState> emit,
  ) async {
    final bool isRefresh = event.refresh || state is! NotificationHistoryLoaded;

    List<NotificationModel> currentHistory = [];
    if (!isRefresh && state is NotificationHistoryLoaded) {
      currentHistory = (state as NotificationHistoryLoaded).history;
    } else {
      emit(NotificationHistoryLoading());
    }

    try {
      final offset = isRefresh ? 0 : currentHistory.length;
      final newItems = await notificationRepository.getHistory(
        tenantId: event.tenantId,
        status: event.status,
        channel: event.channel,
        recipientSearch: event.recipientSearch,
        limit: 50,
        offset: offset,
      );

      emit(
        NotificationHistoryLoaded(
          history: isRefresh
              ? newItems
              : (List.from(currentHistory)..addAll(newItems)),
          hasReachedMax: newItems.length < 50,
        ),
      );
    } catch (e) {
      emit(NotificationHistoryFailure(e.toString()));
    }
  }

  Future<void> _onSendNotification(
    SendNotificationEvent event,
    Emitter<NotificationState> emit,
  ) async {
    emit(NotificationSendProgress());
    try {
      final result = await notificationRepository.sendNotification(
        tenantId: event.tenantId,
        channel: event.channel,
        recipientId: event.recipientId,
        recipientEmail: event.recipientEmail,
        recipientPhone: event.recipientPhone,
        recipientToken: event.recipientToken,
        priority: event.priority,
        templateId: event.templateId,
        subject: event.subject,
        body: event.body,
        metadata: event.metadata,
        idempotencyKey: event.idempotencyKey,
      );
      emit(NotificationSendSuccess(result));
    } catch (e) {
      emit(NotificationSendFailure(e.toString()));
    }
  }
}

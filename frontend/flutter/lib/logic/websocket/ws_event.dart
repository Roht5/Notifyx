import 'package:equatable/equatable.dart';

abstract class WsEvent extends Equatable {
  const WsEvent();

  @override
  List<Object?> get props => [];
}

class ConnectWs extends WsEvent {
  final String tenantId;
  final String userId;
  final String apiKey;

  const ConnectWs({
    required this.tenantId,
    required this.userId,
    required this.apiKey,
  });

  @override
  List<Object?> get props => [tenantId, userId, apiKey];
}

class DisconnectWs extends WsEvent {}

class WsMessageReceivedEvent extends WsEvent {
  final String rawMessage;

  const WsMessageReceivedEvent(this.rawMessage);

  @override
  List<Object?> get props => [rawMessage];
}

class WsErrorOccurredEvent extends WsEvent {
  final String error;

  const WsErrorOccurredEvent(this.error);

  @override
  List<Object?> get props => [error];
}

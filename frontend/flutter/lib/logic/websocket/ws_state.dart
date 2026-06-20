import 'package:equatable/equatable.dart';

abstract class WsState extends Equatable {
  const WsState();

  @override
  List<Object?> get props => [];
}

class WsDisconnected extends WsState {}

class WsConnecting extends WsState {}

class WsConnected extends WsState {
  final List<String> eventsLog;
  final String? latestEvent;

  const WsConnected({required this.eventsLog, this.latestEvent});

  @override
  List<Object?> get props => [eventsLog, latestEvent];
}

class WsConnectionError extends WsState {
  final String error;

  const WsConnectionError(this.error);

  @override
  List<Object?> get props => [error];
}

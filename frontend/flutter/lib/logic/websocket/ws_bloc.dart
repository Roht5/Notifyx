import 'dart:async';
import 'dart:convert';
import 'dart:math';
import 'package:flutter_bloc/flutter_bloc.dart';
import '../../core/network/websocket_client.dart';
import 'ws_event.dart';
import 'ws_state.dart';

class WsBloc extends Bloc<WsEvent, WsState> {
  final WebSocketClient webSocketClient;
  final bool useMock;

  StreamSubscription? _subscription;
  Timer? _mockTimer;
  final List<String> _eventsLog = [];

  WsBloc({required this.webSocketClient, required this.useMock})
    : super(WsDisconnected()) {
    on<ConnectWs>(_onConnect);
    on<DisconnectWs>(_onDisconnect);
    on<WsMessageReceivedEvent>(_onMessageReceived);
    on<WsErrorOccurredEvent>(_onErrorOccurred);
  }

  void _onConnect(ConnectWs event, Emitter<WsState> emit) {
    emit(WsConnecting());
    _eventsLog.clear();

    if (useMock) {
      // Connect instantly in mock mode
      emit(const WsConnected(eventsLog: []));
      _startMockSimulation();
    } else {
      // Connect to real WebSocket server
      webSocketClient.connect(event.tenantId, event.userId, event.apiKey);
      _subscription = webSocketClient.stream.listen(
        (data) => add(WsMessageReceivedEvent(data)),
        onError: (err) => add(WsErrorOccurredEvent(err.toString())),
      );
    }
  }

  void _onDisconnect(DisconnectWs event, Emitter<WsState> emit) {
    _cleanup();
    emit(WsDisconnected());
  }

  void _onMessageReceived(WsMessageReceivedEvent event, Emitter<WsState> emit) {
    _eventsLog.insert(0, event.rawMessage);
    if (_eventsLog.length > 50) {
      _eventsLog.removeLast(); // Keep limit of 50 log items
    }
    emit(
      WsConnected(
        eventsLog: List.from(_eventsLog),
        latestEvent: event.rawMessage,
      ),
    );
  }

  void _onErrorOccurred(WsErrorOccurredEvent event, Emitter<WsState> emit) {
    emit(WsConnectionError(event.error));
  }

  void _cleanup() {
    _subscription?.cancel();
    _subscription = null;
    _mockTimer?.cancel();
    _mockTimer = null;
    if (!useMock) {
      webSocketClient.disconnect();
    }
  }

  // Generate realistic notification event logs dynamically in Mock Mode
  void _startMockSimulation() {
    _mockTimer?.cancel();

    final channels = ['email', 'sms', 'push', 'websocket'];
    final statuses = ['queued', 'delivered', 'failed', 'retrying'];
    final domains = ['gmail.com', 'yahoo.com', 'outlook.com', 'company.org'];
    final names = ['Alice', 'Bob', 'Charlie', 'David', 'Eve'];
    final random = Random();

    _mockTimer = Timer.periodic(const Duration(seconds: 4), (timer) {
      final String channel = channels[random.nextInt(channels.length)];
      final String status = statuses[random.nextInt(statuses.length)];
      final String name = names[random.nextInt(names.length)];
      final String notifId = 'notif-${random.nextInt(9000) + 1000}';

      String recipient = 'user_${random.nextInt(50) + 10}';
      String? errorMessage;

      if (channel == 'email') {
        recipient =
            '${name.toLowerCase()}@${domains[random.nextInt(domains.length)]}';
      } else if (channel == 'sms') {
        recipient = '+9198765${random.nextInt(89999) + 10000}';
      }

      if (status == 'failed') {
        errorMessage = 'Provider Gateway Timeout (Code 504)';
      } else if (status == 'retrying') {
        errorMessage =
            'Rate limit hit, retry attempt #${random.nextInt(2) + 1}';
      }

      final Map<String, dynamic> eventPayload = {
        'id': notifId,
        'channel': channel,
        'status': status,
        'recipient': recipient,
        'body': 'Alert event published for $name',
        'timestamp': DateTime.now().toIso8601String(),
        'error_message': errorMessage,
      };

      if (state is WsConnected) {
        add(WsMessageReceivedEvent(jsonEncode(eventPayload)));
      }
    });
  }

  @override
  Future<void> close() {
    _cleanup();
    return super.close();
  }
}

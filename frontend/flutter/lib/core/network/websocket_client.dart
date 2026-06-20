import 'dart:async';
import 'package:web_socket_channel/web_socket_channel.dart';

class WebSocketClient {
  final String wsBaseUrl;
  WebSocketChannel? _channel;
  StreamController<String>? _streamController;
  StreamSubscription? _subscription;

  WebSocketClient({required this.wsBaseUrl});

  Stream<String> get stream {
    _streamController ??= StreamController<String>.broadcast();
    return _streamController!.stream;
  }

  void connect(String tenantId, String userId, String apiKey) {
    disconnect();

    _streamController ??= StreamController<String>.broadcast();
    final uri = Uri.parse(
      '$wsBaseUrl/ws/connect?tenantId=$tenantId&userId=$userId&apiKey=$apiKey',
    );

    try {
      _channel = WebSocketChannel.connect(uri);
      _subscription = _channel!.stream.listen(
        (data) {
          if (_streamController != null && !_streamController!.isClosed) {
            _streamController!.add(data.toString());
          }
        },
        onError: (err) {
          if (_streamController != null && !_streamController!.isClosed) {
            _streamController!.addError(err);
          }
        },
        onDone: () {
          disconnect();
        },
      );
    } catch (e) {
      if (_streamController != null && !_streamController!.isClosed) {
        _streamController!.addError(e);
      }
    }
  }

  void disconnect() {
    _subscription?.cancel();
    _subscription = null;
    _channel?.sink.close();
    _channel = null;
  }
}

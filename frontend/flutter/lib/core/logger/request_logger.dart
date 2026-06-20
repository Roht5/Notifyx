import 'dart:io';
import 'package:flutter/foundation.dart';

class RequestLogger {
  static void logError(String message) {
    final logLine = '[${DateTime.now().toUtc().toIso8601String()}] $message\n';

    // Always print to console
    debugPrint(logLine);

    // Conditionally write to log.txt on non-Web environments to prevent runtime crashes
    if (!kIsWeb) {
      try {
        final file = File('log.txt');
        file.writeAsStringSync(logLine, mode: FileMode.append);
      } catch (e) {
        debugPrint('Failed to write log to local file: $e');
      }
    }
  }
}

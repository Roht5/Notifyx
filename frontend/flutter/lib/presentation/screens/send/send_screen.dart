import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import '../../../logic/notification/notification_bloc.dart';
import '../../../logic/notification/notification_event.dart';
import '../../../logic/notification/notification_state.dart';
import '../../../logic/tenant/tenant_bloc.dart';
import '../../../logic/tenant/tenant_state.dart';
import '../../../core/theme/app_colors.dart';
import '../../widgets/glass_card.dart';

class SendScreen extends StatefulWidget {
  const SendScreen({super.key});

  @override
  State<SendScreen> createState() => _SendScreenState();
}

class _SendScreenState extends State<SendScreen> {
  String selectedTemplate = 'welcome-email';
  String selectedChannel = 'email';
  String schedule = 'now';
  final TextEditingController _recipientsController = TextEditingController(
    text: 'team@example.com',
  );
  final TextEditingController _bodyController = TextEditingController(
    text: 'Welcome to Notifyx! Your account is ready to go.',
  );
  bool copied = false;
  bool sent = false;
  bool isLoading = false;

  void handleCopy() {
    Clipboard.setData(ClipboardData(text: _getJsonPayload()));
    setState(() => copied = true);
    Future.delayed(const Duration(seconds: 2), () {
      if (mounted) setState(() => copied = false);
    });
  }

  void handleSend() {
    final tenantState = context.read<TenantBloc>().state;
    if (tenantState is! TenantLoaded || tenantState.activeTenant == null) {
      return;
    }
    final tenantId = tenantState.activeTenant!.id;

    final recipients = _recipientsController.text
        .split(RegExp(r'[,\n]'))
        .map((s) => s.trim())
        .where((s) => s.isNotEmpty)
        .toList();
    if (recipients.isEmpty || _bodyController.text.trim().isEmpty) return;

    final bloc = context.read<NotificationBloc>();
    for (final recipient in recipients) {
      bloc.add(
        SendNotificationEvent(
          tenantId: tenantId,
          channel: selectedChannel,
          recipientId: recipient,
          recipientEmail: selectedChannel == 'email' ? recipient : null,
          recipientPhone: selectedChannel == 'sms' ? recipient : null,
          recipientToken:
              (selectedChannel == 'push' || selectedChannel == 'websocket')
              ? recipient
              : null,
          priority: 'normal',
          templateId: selectedTemplate == 'custom' ? null : selectedTemplate,
          body: _bodyController.text.trim(),
          metadata: const {},
        ),
      );
    }
  }

  String _getJsonPayload() {
    final list = _recipientsController.text
        .split(RegExp(r'[,\n]'))
        .where((s) => s.trim().isNotEmpty)
        .toList();
    final map = {
      'template': selectedTemplate,
      'channel': selectedChannel,
      'recipients': list,
      'body': _bodyController.text,
    };
    return const JsonEncoder.withIndent('  ').convert(map);
  }

  @override
  Widget build(BuildContext context) {
    return BlocListener<NotificationBloc, NotificationState>(
      listener: (context, state) {
        if (state is NotificationSendProgress) {
          setState(() => isLoading = true);
        } else if (state is NotificationSendSuccess) {
          setState(() {
            isLoading = false;
            sent = true;
          });
          Future.delayed(const Duration(seconds: 3), () {
            if (mounted) setState(() => sent = false);
          });
        } else if (state is NotificationSendFailure) {
          setState(() => isLoading = false);
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(content: Text('Send failed: ${state.error}')),
          );
        }
      },
      child: Scaffold(
        backgroundColor: Colors.transparent,
        body: SingleChildScrollView(
          padding: const EdgeInsets.all(24.0),
          child: LayoutBuilder(
            builder: (context, constraints) {
              final isDesktop = constraints.maxWidth > 900;
              return Flex(
                direction: isDesktop ? Axis.horizontal : Axis.vertical,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Expanded(flex: isDesktop ? 2 : 0, child: _buildFormColumn()),
                  if (isDesktop) const SizedBox(width: 32),
                  if (!isDesktop) const SizedBox(height: 32),
                  Expanded(
                    flex: isDesktop ? 1 : 0,
                    child: _buildPreviewColumn(),
                  ),
                ],
              );
            },
          ),
        ),
      ),
    );
  }

  Widget _buildFormColumn() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        // Template Selection
        GlassCard(
          padding: const EdgeInsets.all(24),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const Text(
                'Select Template',
                style: TextStyle(
                  color: AppColors.textPrimary,
                  fontSize: 18,
                  fontWeight: FontWeight.w600,
                ),
              ),
              const SizedBox(height: 16),
              ...[
                'welcome-email',
                'order-confirmation',
                'password-reset',
                'custom',
              ].map(
                (t) => _buildRadioTile(
                  title: t.replaceAll('-', ' '),
                  value: t,
                  groupValue: selectedTemplate,
                  onChanged: (val) => setState(() => selectedTemplate = val!),
                ),
              ),
            ],
          ),
        ),
        const SizedBox(height: 24),

        // Channel Selection
        GlassCard(
          padding: const EdgeInsets.all(24),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const Text(
                'Channel',
                style: TextStyle(
                  color: AppColors.textPrimary,
                  fontSize: 18,
                  fontWeight: FontWeight.w600,
                ),
              ),
              const SizedBox(height: 16),
              Wrap(
                spacing: 12,
                runSpacing: 12,
                children:
                    [
                          {'id': 'email', 'label': 'Email'},
                          {'id': 'sms', 'label': 'SMS'},
                          {'id': 'push', 'label': 'Push'},
                          {'id': 'websocket', 'label': 'In-App'},
                        ]
                        .map((c) => _buildChannelButton(c['id']!, c['label']!))
                        .toList(),
              ),
            ],
          ),
        ),
        const SizedBox(height: 24),

        // Recipients
        GlassCard(
          padding: const EdgeInsets.all(24),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const Text(
                'Recipients',
                style: TextStyle(
                  color: AppColors.textPrimary,
                  fontSize: 18,
                  fontWeight: FontWeight.w600,
                ),
              ),
              const SizedBox(height: 16),
              Container(
                decoration: BoxDecoration(
                  color: AppColors.surfaceObsidian.withOpacity(0.5),
                  border: Border.all(color: AppColors.borderSubtle),
                  borderRadius: BorderRadius.circular(8),
                ),
                child: TextField(
                  controller: _recipientsController,
                  maxLines: 4,
                  onChanged: (_) => setState(() {}),
                  style: const TextStyle(color: AppColors.textPrimary),
                  decoration: const InputDecoration(
                    border: InputBorder.none,
                    contentPadding: EdgeInsets.all(16),
                    hintText: 'team@example.com, user@example.com, ...',
                    hintStyle: TextStyle(color: AppColors.textMuted),
                  ),
                ),
              ),
              const SizedBox(height: 8),
              const Text(
                'Enter email addresses, one per line or comma-separated',
                style: TextStyle(color: AppColors.textSecondary, fontSize: 12),
              ),
            ],
          ),
        ),
        const SizedBox(height: 24),

        // Message Body
        GlassCard(
          padding: const EdgeInsets.all(24),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const Text(
                'Message',
                style: TextStyle(
                  color: AppColors.textPrimary,
                  fontSize: 18,
                  fontWeight: FontWeight.w600,
                ),
              ),
              const SizedBox(height: 16),
              Container(
                decoration: BoxDecoration(
                  color: AppColors.surfaceObsidian.withOpacity(0.5),
                  border: Border.all(color: AppColors.borderSubtle),
                  borderRadius: BorderRadius.circular(8),
                ),
                child: TextField(
                  controller: _bodyController,
                  maxLines: 3,
                  onChanged: (_) => setState(() {}),
                  style: const TextStyle(color: AppColors.textPrimary),
                  decoration: const InputDecoration(
                    border: InputBorder.none,
                    contentPadding: EdgeInsets.all(16),
                    hintText: 'Notification body text...',
                    hintStyle: TextStyle(color: AppColors.textMuted),
                  ),
                ),
              ),
            ],
          ),
        ),
        const SizedBox(height: 24),

        // Scheduling
        GlassCard(
          padding: const EdgeInsets.all(24),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const Text(
                'Schedule',
                style: TextStyle(
                  color: AppColors.textPrimary,
                  fontSize: 18,
                  fontWeight: FontWeight.w600,
                ),
              ),
              const SizedBox(height: 16),
              ...['now', 'scheduled', 'recurring'].map(
                (s) => _buildRadioTile(
                  title: s,
                  value: s,
                  groupValue: schedule,
                  onChanged: (val) => setState(() => schedule = val!),
                ),
              ),
            ],
          ),
        ),
        const SizedBox(height: 24),

        // Send Button
        GestureDetector(
          onTap: isLoading ? null : handleSend,
          child: AnimatedContainer(
            duration: const Duration(milliseconds: 200),
            width: double.infinity,
            padding: const EdgeInsets.symmetric(vertical: 16),
            decoration: BoxDecoration(
              gradient: LinearGradient(
                colors: isLoading
                    ? [AppColors.textMuted, AppColors.textMuted]
                    : [AppColors.neonTeal, AppColors.electricViolet],
              ),
              borderRadius: BorderRadius.circular(8),
            ),
            child: Center(
              child: isLoading
                  ? const SizedBox(
                      width: 20,
                      height: 20,
                      child: CircularProgressIndicator(
                        strokeWidth: 2,
                        color: Colors.white,
                      ),
                    )
                  : const Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(Icons.send, color: Colors.black, size: 20),
                        SizedBox(width: 8),
                        Text(
                          'Send Notification',
                          style: TextStyle(
                            color: Colors.black,
                            fontSize: 16,
                            fontWeight: FontWeight.bold,
                          ),
                        ),
                      ],
                    ),
            ),
          ),
        ),
        if (sent) ...[
          const SizedBox(height: 16),
          Container(
            padding: const EdgeInsets.all(16),
            decoration: BoxDecoration(
              color: AppColors.emeraldGreen.withOpacity(0.1),
              border: Border.all(
                color: AppColors.emeraldGreen.withOpacity(0.3),
              ),
              borderRadius: BorderRadius.circular(8),
            ),
            child: Row(
              children: [
                const Icon(Icons.check, color: AppColors.emeraldGreen),
                const SizedBox(width: 8),
                Text(
                  'Notification sent successfully to ${_recipientsController.text.split(RegExp(r'[,\n]')).where((s) => s.trim().isNotEmpty).length} recipient(s)',
                  style: const TextStyle(
                    color: AppColors.emeraldGreen,
                    fontWeight: FontWeight.w500,
                  ),
                ),
              ],
            ),
          ),
        ],
      ],
    );
  }

  Widget _buildPreviewColumn() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        GlassCard(
          padding: const EdgeInsets.all(24),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  const Text(
                    'JSON Payload',
                    style: TextStyle(
                      color: AppColors.textPrimary,
                      fontSize: 18,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                  IconButton(
                    icon: Icon(
                      copied ? Icons.check : Icons.copy,
                      color: copied
                          ? AppColors.emeraldGreen
                          : AppColors.textMuted,
                      size: 20,
                    ),
                    onPressed: handleCopy,
                  ),
                ],
              ),
              const SizedBox(height: 16),
              Container(
                width: double.infinity,
                padding: const EdgeInsets.all(16),
                decoration: BoxDecoration(
                  color: AppColors.surfaceObsidian.withOpacity(0.5),
                  border: Border.all(color: AppColors.borderSubtle),
                  borderRadius: BorderRadius.circular(8),
                ),
                child: Text(
                  _getJsonPayload(),
                  style: const TextStyle(
                    color: AppColors.neonTeal,
                    fontFamily: 'monospace',
                    fontSize: 12,
                  ),
                ),
              ),
            ],
          ),
        ),
        const SizedBox(height: 24),
        GlassCard(
          padding: const EdgeInsets.all(24),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const Text(
                'Estimated Impact',
                style: TextStyle(
                  color: AppColors.textPrimary,
                  fontSize: 18,
                  fontWeight: FontWeight.w600,
                ),
              ),
              const SizedBox(height: 16),
              _buildImpactRow(
                'Recipients',
                _recipientsController.text
                    .split(RegExp(r'[,\n]'))
                    .where((s) => s.trim().isNotEmpty)
                    .length
                    .toString(),
                AppColors.textPrimary,
              ),
              const SizedBox(height: 12),
              _buildImpactRow('Estimated Cost', '\$0.02', AppColors.neonTeal),
              const SizedBox(height: 12),
              _buildImpactRow('Est. Time', '~2 min', AppColors.textPrimary),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildRadioTile({
    required String title,
    required String value,
    required String groupValue,
    required ValueChanged<String?> onChanged,
  }) {
    final isSelected = value == groupValue;
    return GestureDetector(
      onTap: () => onChanged(value),
      child: Container(
        margin: const EdgeInsets.only(bottom: 8),
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
        decoration: BoxDecoration(
          color: isSelected
              ? AppColors.neonTeal.withOpacity(0.1)
              : AppColors.surfaceObsidian.withOpacity(0.5),
          border: Border.all(
            color: isSelected ? AppColors.neonTeal : AppColors.borderSubtle,
            width: 2,
          ),
          borderRadius: BorderRadius.circular(8),
        ),
        child: Row(
          children: [
            Radio<String>(
              value: value,
              groupValue: groupValue,
              onChanged: onChanged,
              activeColor: AppColors.neonTeal,
            ),
            Text(
              title,
              style: const TextStyle(
                color: AppColors.textPrimary,
                fontWeight: FontWeight.w500,
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildChannelButton(String id, String label) {
    final isSelected = selectedChannel == id;
    return GestureDetector(
      onTap: () => setState(() => selectedChannel = id),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 12),
        decoration: BoxDecoration(
          color: isSelected
              ? AppColors.neonTeal.withOpacity(0.2)
              : AppColors.surfaceObsidian.withOpacity(0.3),
          border: Border.all(
            color: isSelected ? AppColors.neonTeal : AppColors.borderSubtle,
            width: 2,
          ),
          borderRadius: BorderRadius.circular(8),
        ),
        child: Text(
          label,
          style: TextStyle(
            color: isSelected ? AppColors.neonTeal : AppColors.textSecondary,
            fontWeight: FontWeight.w600,
          ),
        ),
      ),
    );
  }

  Widget _buildImpactRow(String label, String value, Color valueColor) {
    return Row(
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
      children: [
        Text(
          label,
          style: const TextStyle(color: AppColors.textSecondary, fontSize: 14),
        ),
        Text(
          value,
          style: TextStyle(
            color: valueColor,
            fontSize: 14,
            fontWeight: FontWeight.bold,
          ),
        ),
      ],
    );
  }
}

import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import '../../../data/models/template_model.dart';
import '../../../logic/template/template_bloc.dart';
import '../../../logic/template/template_event.dart';
import '../../../logic/template/template_state.dart';
import '../../../logic/tenant/tenant_bloc.dart';
import '../../../logic/tenant/tenant_state.dart';
import '../../../core/theme/app_colors.dart';
import '../../widgets/glass_card.dart';

class TemplatesScreen extends StatefulWidget {
  const TemplatesScreen({super.key});

  @override
  State<TemplatesScreen> createState() => _TemplatesScreenState();
}

class _TemplatesScreenState extends State<TemplatesScreen> {
  String? _loadedTenantId;

  void _showTemplateModal(String tenantId, {TemplateModel? existing}) {
    final nameController = TextEditingController(text: existing?.name ?? '');
    final subjectController = TextEditingController(
      text: existing?.subject ?? '',
    );
    final bodyController = TextEditingController(text: existing?.body ?? '');
    String channel = existing?.channel ?? 'email';

    showDialog(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (dialogContext, setDialogState) => Dialog(
          backgroundColor: Colors.transparent,
          child: GlassCard(
            padding: const EdgeInsets.all(24),
            child: SizedBox(
              width: 420,
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    children: [
                      Text(
                        existing == null
                            ? 'Create New Template'
                            : 'Edit Template',
                        style: const TextStyle(
                          color: AppColors.textPrimary,
                          fontSize: 18,
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                      IconButton(
                        icon: const Icon(
                          Icons.close,
                          color: AppColors.textMuted,
                        ),
                        onPressed: () => Navigator.pop(dialogContext),
                      ),
                    ],
                  ),
                  const SizedBox(height: 16),
                  _buildInput(nameController, 'Template name'),
                  const SizedBox(height: 12),
                  Container(
                    padding: const EdgeInsets.symmetric(horizontal: 12),
                    decoration: BoxDecoration(
                      color: AppColors.surfaceObsidian.withOpacity(0.5),
                      border: Border.all(color: AppColors.borderSubtle),
                      borderRadius: BorderRadius.circular(8),
                    ),
                    child: DropdownButtonHideUnderline(
                      child: DropdownButton<String>(
                        value: channel,
                        isExpanded: true,
                        dropdownColor: AppColors.surfaceObsidian,
                        style: const TextStyle(color: AppColors.textPrimary),
                        items: const [
                          DropdownMenuItem(value: 'email', child: Text('Email')),
                          DropdownMenuItem(value: 'sms', child: Text('SMS')),
                          DropdownMenuItem(value: 'push', child: Text('Push')),
                          DropdownMenuItem(
                            value: 'websocket',
                            child: Text('In-App'),
                          ),
                        ],
                        onChanged: (val) {
                          if (val != null) {
                            setDialogState(() => channel = val);
                          }
                        },
                      ),
                    ),
                  ),
                  const SizedBox(height: 12),
                  _buildInput(subjectController, 'Subject (optional)'),
                  const SizedBox(height: 12),
                  _buildInput(bodyController, 'Body ({{variables}} allowed)',
                      maxLines: 4),
                  const SizedBox(height: 24),
                  Row(
                    children: [
                      Expanded(
                        child: GestureDetector(
                          onTap: () => Navigator.pop(dialogContext),
                          child: Container(
                            padding: const EdgeInsets.symmetric(vertical: 12),
                            decoration: BoxDecoration(
                              color: AppColors.surfaceObsidian.withOpacity(0.5),
                              border: Border.all(color: AppColors.borderSubtle),
                              borderRadius: BorderRadius.circular(8),
                            ),
                            alignment: Alignment.center,
                            child: const Text(
                              'Cancel',
                              style: TextStyle(
                                color: AppColors.textPrimary,
                                fontWeight: FontWeight.w500,
                              ),
                            ),
                          ),
                        ),
                      ),
                      const SizedBox(width: 16),
                      Expanded(
                        child: GestureDetector(
                          onTap: () {
                            if (nameController.text.trim().isEmpty ||
                                bodyController.text.trim().isEmpty) {
                              return;
                            }
                            final bloc = context.read<TemplateBloc>();
                            if (existing == null) {
                              bloc.add(
                                CreateTemplateEvent(
                                  tenantId: tenantId,
                                  template: TemplateModel(
                                    id: '',
                                    tenantId: tenantId,
                                    name: nameController.text.trim(),
                                    channel: channel,
                                    subject: subjectController.text.trim().isEmpty
                                        ? null
                                        : subjectController.text.trim(),
                                    body: bodyController.text.trim(),
                                    createdAt: DateTime.now(),
                                  ),
                                ),
                              );
                            } else {
                              bloc.add(
                                UpdateTemplateEvent(
                                  tenantId: tenantId,
                                  template: existing.copyWith(
                                    name: nameController.text.trim(),
                                    channel: channel,
                                    subject: subjectController.text.trim().isEmpty
                                        ? null
                                        : subjectController.text.trim(),
                                    body: bodyController.text.trim(),
                                  ),
                                ),
                              );
                            }
                            Navigator.pop(dialogContext);
                          },
                          child: Container(
                            padding: const EdgeInsets.symmetric(vertical: 12),
                            decoration: BoxDecoration(
                              color: AppColors.neonTeal,
                              borderRadius: BorderRadius.circular(8),
                            ),
                            alignment: Alignment.center,
                            child: Text(
                              existing == null ? 'Create' : 'Save',
                              style: const TextStyle(
                                color: Colors.black,
                                fontWeight: FontWeight.bold,
                              ),
                            ),
                          ),
                        ),
                      ),
                    ],
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildInput(
    TextEditingController controller,
    String hint, {
    int maxLines = 1,
  }) {
    return Container(
      decoration: BoxDecoration(
        color: AppColors.surfaceObsidian.withOpacity(0.5),
        border: Border.all(color: AppColors.borderSubtle),
        borderRadius: BorderRadius.circular(8),
      ),
      child: TextField(
        controller: controller,
        maxLines: maxLines,
        style: const TextStyle(color: AppColors.textPrimary),
        decoration: InputDecoration(
          border: InputBorder.none,
          contentPadding: const EdgeInsets.symmetric(
            horizontal: 16,
            vertical: 12,
          ),
          hintText: hint,
          hintStyle: const TextStyle(color: AppColors.textMuted),
        ),
      ),
    );
  }

  void _showViewTemplateModal(TemplateModel template) {
    showDialog(
      context: context,
      builder: (context) => Dialog(
        backgroundColor: Colors.transparent,
        child: GlassCard(
          padding: const EdgeInsets.all(24),
          child: SizedBox(
            width: 400,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  mainAxisAlignment: MainAxisAlignment.spaceBetween,
                  children: [
                    const Text(
                      'View Template',
                      style: TextStyle(
                        color: AppColors.textPrimary,
                        fontSize: 18,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    IconButton(
                      icon: const Icon(Icons.close, color: AppColors.textMuted),
                      onPressed: () => Navigator.pop(context),
                    ),
                  ],
                ),
                const SizedBox(height: 16),
                _buildInfoRow('Name', template.name),
                const SizedBox(height: 12),
                _buildInfoRow('Channel', template.channel),
                if (template.subject != null) ...[
                  const SizedBox(height: 12),
                  _buildInfoRow('Subject', template.subject!),
                ],
                const SizedBox(height: 12),
                _buildInfoRow('Body', template.body),
                const SizedBox(height: 24),
                GestureDetector(
                  onTap: () => Navigator.pop(context),
                  child: Container(
                    width: double.infinity,
                    padding: const EdgeInsets.symmetric(vertical: 12),
                    decoration: BoxDecoration(
                      color: AppColors.neonTeal,
                      borderRadius: BorderRadius.circular(8),
                    ),
                    alignment: Alignment.center,
                    child: const Text(
                      'Close',
                      style: TextStyle(
                        color: Colors.black,
                        fontWeight: FontWeight.bold,
                      ),
                    ),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildInfoRow(String label, String value) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          label,
          style: const TextStyle(color: AppColors.textSecondary, fontSize: 12),
        ),
        const SizedBox(height: 4),
        Text(
          value,
          style: const TextStyle(
            color: AppColors.textPrimary,
            fontWeight: FontWeight.w500,
          ),
        ),
      ],
    );
  }

  @override
  Widget build(BuildContext context) {
    return BlocBuilder<TenantBloc, TenantState>(
      builder: (context, tenantState) {
        if (tenantState is! TenantLoaded || tenantState.activeTenant == null) {
          return const SizedBox.shrink();
        }
        final tenantId = tenantState.activeTenant!.id;
        if (_loadedTenantId != tenantId) {
          _loadedTenantId = tenantId;
          context.read<TemplateBloc>().add(LoadTemplates(tenantId));
        }

        return BlocBuilder<TemplateBloc, TemplateState>(
          builder: (context, state) {
            return Scaffold(
              backgroundColor: Colors.transparent,
              body: SingleChildScrollView(
                padding: const EdgeInsets.all(24.0),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      mainAxisAlignment: MainAxisAlignment.spaceBetween,
                      children: [
                        const Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(
                              'Message Templates',
                              style: TextStyle(
                                color: AppColors.textPrimary,
                                fontSize: 24,
                                fontWeight: FontWeight.bold,
                              ),
                            ),
                            SizedBox(height: 4),
                            Text(
                              'Manage your notification templates',
                              style: TextStyle(
                                color: AppColors.textSecondary,
                                fontSize: 14,
                              ),
                            ),
                          ],
                        ),
                        GestureDetector(
                          onTap: () => _showTemplateModal(tenantId),
                          child: Container(
                            padding: const EdgeInsets.symmetric(
                              horizontal: 16,
                              vertical: 12,
                            ),
                            decoration: BoxDecoration(
                              gradient: const LinearGradient(
                                colors: [
                                  AppColors.neonTeal,
                                  AppColors.electricViolet,
                                ],
                              ),
                              borderRadius: BorderRadius.circular(8),
                            ),
                            child: const Row(
                              mainAxisSize: MainAxisSize.min,
                              children: [
                                Icon(Icons.add, color: Colors.black, size: 20),
                                SizedBox(width: 8),
                                Text(
                                  'New Template',
                                  style: TextStyle(
                                    color: Colors.black,
                                    fontWeight: FontWeight.bold,
                                  ),
                                ),
                              ],
                            ),
                          ),
                        ),
                      ],
                    ),
                    const SizedBox(height: 24),
                    if (state is TemplateLoading)
                      const Padding(
                        padding: EdgeInsets.symmetric(vertical: 48),
                        child: Center(
                          child: CircularProgressIndicator(
                            color: AppColors.neonTeal,
                          ),
                        ),
                      )
                    else if (state is TemplateFailure)
                      GlassCard(
                        padding: const EdgeInsets.all(24),
                        child: Text(
                          'Failed to load templates: ${state.error}',
                          style: const TextStyle(color: AppColors.coralRed),
                        ),
                      )
                    else if (state is TemplateLoaded)
                      LayoutBuilder(
                        builder: (context, constraints) {
                          final isDesktop = constraints.maxWidth > 900;
                          final isTablet = constraints.maxWidth > 600;
                          final crossAxisCount =
                              isDesktop ? 3 : (isTablet ? 2 : 1);

                          if (state.templates.isEmpty) {
                            return GlassCard(
                              padding: const EdgeInsets.all(24),
                              child: const Text(
                                'No templates yet. Create one to get started.',
                                style: TextStyle(
                                  color: AppColors.textSecondary,
                                ),
                              ),
                            );
                          }

                          return GridView.builder(
                            shrinkWrap: true,
                            physics: const NeverScrollableScrollPhysics(),
                            gridDelegate:
                                SliverGridDelegateWithFixedCrossAxisCount(
                              crossAxisCount: crossAxisCount,
                              crossAxisSpacing: 24,
                              mainAxisSpacing: 24,
                              childAspectRatio: 1.2,
                            ),
                            itemCount: state.templates.length,
                            itemBuilder: (context, index) {
                              final template = state.templates[index];
                              return _buildTemplateCard(tenantId, template);
                            },
                          );
                        },
                      ),
                  ],
                ),
              ),
            );
          },
        );
      },
    );
  }

  Widget _buildTemplateCard(String tenantId, TemplateModel template) {
    return GlassCard(
      padding: const EdgeInsets.all(24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            template.name,
            style: const TextStyle(
              color: AppColors.textPrimary,
              fontSize: 18,
              fontWeight: FontWeight.w600,
            ),
          ),
          const SizedBox(height: 4),
          Text(
            template.subject ?? template.body,
            style: const TextStyle(
              color: AppColors.textSecondary,
              fontSize: 14,
            ),
            maxLines: 2,
            overflow: TextOverflow.ellipsis,
          ),
          const SizedBox(height: 16),
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: [
              Container(
                padding: const EdgeInsets.symmetric(
                  horizontal: 12,
                  vertical: 4,
                ),
                decoration: BoxDecoration(
                  color: AppColors.neonTeal.withOpacity(0.1),
                  borderRadius: BorderRadius.circular(16),
                ),
                child: Text(
                  template.channel,
                  style: const TextStyle(
                    color: AppColors.neonTeal,
                    fontSize: 12,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ),
            ],
          ),
          const Spacer(),
          Text(
            'Created ${template.createdAt.toLocal().toString().split(' ')[0]}',
            style: const TextStyle(color: AppColors.textMuted, fontSize: 12),
          ),
          const SizedBox(height: 16),
          Row(
            children: [
              Expanded(
                child: GestureDetector(
                  onTap: () => _showViewTemplateModal(template),
                  child: Container(
                    padding: const EdgeInsets.symmetric(vertical: 8),
                    decoration: BoxDecoration(
                      color: AppColors.surfaceObsidian.withOpacity(0.5),
                      border: Border.all(color: AppColors.borderSubtle),
                      borderRadius: BorderRadius.circular(8),
                    ),
                    child: const Row(
                      mainAxisAlignment: MainAxisAlignment.center,
                      children: [
                        Icon(
                          Icons.visibility,
                          size: 16,
                          color: AppColors.textPrimary,
                        ),
                        SizedBox(width: 4),
                        Text(
                          'View',
                          style: TextStyle(
                            color: AppColors.textPrimary,
                            fontSize: 12,
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
              ),
              const SizedBox(width: 8),
              Expanded(
                child: GestureDetector(
                  onTap: () => _showTemplateModal(tenantId, existing: template),
                  child: Container(
                    padding: const EdgeInsets.symmetric(vertical: 8),
                    decoration: BoxDecoration(
                      color: AppColors.surfaceObsidian.withOpacity(0.5),
                      border: Border.all(color: AppColors.borderSubtle),
                      borderRadius: BorderRadius.circular(8),
                    ),
                    child: const Row(
                      mainAxisAlignment: MainAxisAlignment.center,
                      children: [
                        Icon(
                          Icons.edit,
                          size: 16,
                          color: AppColors.textPrimary,
                        ),
                        SizedBox(width: 4),
                        Text(
                          'Edit',
                          style: TextStyle(
                            color: AppColors.textPrimary,
                            fontSize: 12,
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
              ),
              const SizedBox(width: 8),
              GestureDetector(
                onTap: () {
                  context.read<TemplateBloc>().add(
                    DeleteTemplateEvent(
                      tenantId: tenantId,
                      templateId: template.id,
                    ),
                  );
                },
                child: Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: 12,
                    vertical: 8,
                  ),
                  decoration: BoxDecoration(
                    color: AppColors.coralRed.withOpacity(0.1),
                    border: Border.all(
                      color: AppColors.coralRed.withOpacity(0.3),
                    ),
                    borderRadius: BorderRadius.circular(8),
                  ),
                  child: const Icon(
                    Icons.delete_outline,
                    size: 16,
                    color: AppColors.coralRed,
                  ),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

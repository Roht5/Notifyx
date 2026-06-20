import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import '../../../data/models/tenant_model.dart';
import '../../../logic/tenant/tenant_bloc.dart';
import '../../../logic/tenant/tenant_event.dart';
import '../../../logic/tenant/tenant_state.dart';
import '../../../core/theme/app_colors.dart';
import '../../widgets/glass_card.dart';

const List<String> kAllChannels = ['email', 'sms', 'push', 'websocket'];

class TenantsScreen extends StatefulWidget {
  const TenantsScreen({super.key});

  @override
  State<TenantsScreen> createState() => _TenantsScreenState();
}

class _TenantsScreenState extends State<TenantsScreen> {
  void _showAddTenantModal() {
    final nameController = TextEditingController();
    final rateCapController = TextEditingController(text: '500');

    showDialog(
      context: context,
      builder: (dialogContext) => Dialog(
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
                      'Add New Tenant',
                      style: TextStyle(
                        color: AppColors.textPrimary,
                        fontSize: 18,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    IconButton(
                      icon: const Icon(Icons.close, color: AppColors.textMuted),
                      onPressed: () => Navigator.pop(dialogContext),
                    ),
                  ],
                ),
                const SizedBox(height: 16),
                _buildTextField(nameController, 'Tenant name'),
                const SizedBox(height: 16),
                _buildTextField(
                  rateCapController,
                  'Global rate cap (per minute)',
                  isNumeric: true,
                ),
                const SizedBox(height: 24),
                Row(
                  children: [
                    Expanded(
                      child: _buildSecondaryButton(
                        'Cancel',
                        () => Navigator.pop(dialogContext),
                      ),
                    ),
                    const SizedBox(width: 16),
                    Expanded(
                      child: _buildPrimaryButton('Create', () {
                        final name = nameController.text.trim();
                        final cap =
                            int.tryParse(rateCapController.text.trim()) ?? 500;
                        if (name.isEmpty) return;
                        context.read<TenantBloc>().add(
                          CreateNewTenant(name: name, globalRateCap: cap),
                        );
                        Navigator.pop(dialogContext);
                      }),
                    ),
                  ],
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  void _showSettingsModal(TenantModel tenant) {
    final rateCapControllers = <String, TextEditingController>{
      for (final c in kAllChannels)
        c: TextEditingController(
          text: (tenant.channelRateLimits[c] ?? 0).toString(),
        ),
    };
    final enabled = Set<String>.from(tenant.enabledChannels);

    showDialog(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (dialogContext, setDialogState) => Dialog(
          backgroundColor: Colors.transparent,
          child: GlassCard(
            padding: const EdgeInsets.all(24),
            child: SizedBox(
              width: 440,
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    children: [
                      Text(
                        '${tenant.name} Settings',
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
                  const Text(
                    'Enabled Channels & Rate Limits',
                    style: TextStyle(color: AppColors.textPrimary, fontSize: 14),
                  ),
                  const SizedBox(height: 8),
                  ...kAllChannels.map((channel) {
                    final isEnabled = enabled.contains(channel);
                    return Padding(
                      padding: const EdgeInsets.only(bottom: 8),
                      child: Row(
                        children: [
                          Checkbox(
                            value: isEnabled,
                            activeColor: AppColors.neonTeal,
                            onChanged: (val) {
                              setDialogState(() {
                                if (val == true) {
                                  enabled.add(channel);
                                } else {
                                  enabled.remove(channel);
                                }
                              });
                            },
                          ),
                          SizedBox(
                            width: 80,
                            child: Text(
                              channel,
                              style: const TextStyle(
                                color: AppColors.textPrimary,
                              ),
                            ),
                          ),
                          Expanded(
                            child: _buildTextField(
                              rateCapControllers[channel]!,
                              'limit/min',
                              isNumeric: true,
                              dense: true,
                            ),
                          ),
                        ],
                      ),
                    );
                  }),
                  const SizedBox(height: 16),
                  const Text(
                    'API Keys',
                    style: TextStyle(color: AppColors.textPrimary, fontSize: 14),
                  ),
                  const SizedBox(height: 8),
                  ...tenant.apiKeys.map(
                    (key) => Padding(
                      padding: const EdgeInsets.only(bottom: 8),
                      child: Row(
                        children: [
                          Expanded(
                            child: Text(
                              key,
                              style: const TextStyle(
                                color: AppColors.textSecondary,
                                fontFamily: 'monospace',
                                fontSize: 12,
                              ),
                              overflow: TextOverflow.ellipsis,
                            ),
                          ),
                          IconButton(
                            icon: const Icon(
                              Icons.delete_outline,
                              size: 18,
                              color: AppColors.coralRed,
                            ),
                            onPressed: () {
                              context.read<TenantBloc>().add(
                                RevokeTenantApiKey(
                                  tenantId: tenant.id,
                                  keyId: key,
                                ),
                              );
                              Navigator.pop(dialogContext);
                            },
                          ),
                        ],
                      ),
                    ),
                  ),
                  GestureDetector(
                    onTap: () {
                      context.read<TenantBloc>().add(
                        RotateTenantApiKey(tenant.id),
                      );
                      Navigator.pop(dialogContext);
                    },
                    child: const Row(
                      children: [
                        Icon(Icons.refresh, size: 16, color: AppColors.neonTeal),
                        SizedBox(width: 8),
                        Text(
                          'Rotate / Add Key',
                          style: TextStyle(
                            color: AppColors.neonTeal,
                            fontWeight: FontWeight.w500,
                          ),
                        ),
                      ],
                    ),
                  ),
                  const SizedBox(height: 24),
                  Row(
                    children: [
                      Expanded(
                        child: _buildSecondaryButton(
                          'Cancel',
                          () => Navigator.pop(dialogContext),
                        ),
                      ),
                      const SizedBox(width: 16),
                      Expanded(
                        child: _buildPrimaryButton('Save', () {
                          final limits = {
                            for (final c in kAllChannels)
                              c: int.tryParse(
                                    rateCapControllers[c]!.text.trim(),
                                  ) ??
                                  0,
                          };
                          context.read<TenantBloc>().add(
                            UpdateTenantChannelsEvent(
                              tenantId: tenant.id,
                              channels: enabled.toList(),
                            ),
                          );
                          context.read<TenantBloc>().add(
                            UpdateTenantLimitsEvent(
                              tenantId: tenant.id,
                              limits: limits,
                            ),
                          );
                          Navigator.pop(dialogContext);
                        }),
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

  Widget _buildTextField(
    TextEditingController controller,
    String hint, {
    bool isNumeric = false,
    bool dense = false,
  }) {
    return Container(
      padding: EdgeInsets.symmetric(horizontal: 16, vertical: dense ? 0 : 4),
      decoration: BoxDecoration(
        color: AppColors.surfaceObsidian.withOpacity(0.5),
        border: Border.all(color: AppColors.borderSubtle),
        borderRadius: BorderRadius.circular(8),
      ),
      child: TextField(
        controller: controller,
        keyboardType: isNumeric ? TextInputType.number : TextInputType.text,
        style: const TextStyle(color: AppColors.textPrimary),
        decoration: InputDecoration(
          border: InputBorder.none,
          hintText: hint,
          hintStyle: const TextStyle(color: AppColors.textMuted),
        ),
      ),
    );
  }

  Widget _buildSecondaryButton(String label, VoidCallback onTap) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        padding: const EdgeInsets.symmetric(vertical: 12),
        decoration: BoxDecoration(
          color: AppColors.surfaceObsidian.withOpacity(0.5),
          border: Border.all(color: AppColors.borderSubtle),
          borderRadius: BorderRadius.circular(8),
        ),
        alignment: Alignment.center,
        child: Text(
          label,
          style: const TextStyle(
            color: AppColors.textPrimary,
            fontWeight: FontWeight.w500,
          ),
        ),
      ),
    );
  }

  Widget _buildPrimaryButton(String label, VoidCallback onTap) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        padding: const EdgeInsets.symmetric(vertical: 12),
        decoration: BoxDecoration(
          color: AppColors.neonTeal,
          borderRadius: BorderRadius.circular(8),
        ),
        alignment: Alignment.center,
        child: Text(
          label,
          style: const TextStyle(
            color: Colors.black,
            fontWeight: FontWeight.bold,
          ),
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return BlocBuilder<TenantBloc, TenantState>(
      builder: (context, state) {
        if (state is TenantLoading || state is TenantInitial) {
          return const Center(
            child: CircularProgressIndicator(color: AppColors.neonTeal),
          );
        }
        if (state is TenantFailure) {
          return Center(
            child: Text(
              'Failed to load tenants: ${state.error}',
              style: const TextStyle(color: AppColors.coralRed),
            ),
          );
        }

        final tenants = (state as TenantLoaded).tenants;

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
                          'Tenant Management',
                          style: TextStyle(
                            color: AppColors.textPrimary,
                            fontSize: 24,
                            fontWeight: FontWeight.bold,
                          ),
                        ),
                        SizedBox(height: 4),
                        Text(
                          'Manage your organization\'s tenants',
                          style: TextStyle(
                            color: AppColors.textSecondary,
                            fontSize: 14,
                          ),
                        ),
                      ],
                    ),
                    GestureDetector(
                      onTap: _showAddTenantModal,
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
                              'Add Tenant',
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
                LayoutBuilder(
                  builder: (context, constraints) {
                    final isDesktop = constraints.maxWidth > 600;
                    return Flex(
                      direction: isDesktop ? Axis.horizontal : Axis.vertical,
                      children: [
                        Expanded(
                          flex: isDesktop ? 1 : 0,
                          child: _buildStatCard(
                            'Total Tenants',
                            tenants.length.toString(),
                            AppColors.textPrimary,
                          ),
                        ),
                        if (isDesktop) const SizedBox(width: 16),
                        if (!isDesktop) const SizedBox(height: 16),
                        Expanded(
                          flex: isDesktop ? 1 : 0,
                          child: _buildStatCard(
                            'Total API Keys',
                            tenants
                                .fold<int>(0, (s, t) => s + t.apiKeys.length)
                                .toString(),
                            AppColors.emeraldGreen,
                          ),
                        ),
                        if (isDesktop) const SizedBox(width: 16),
                        if (!isDesktop) const SizedBox(height: 16),
                        Expanded(
                          flex: isDesktop ? 1 : 0,
                          child: _buildStatCard(
                            'Combined Rate Cap',
                            tenants
                                .fold<int>(0, (s, t) => s + t.globalRateCap)
                                .toString(),
                            AppColors.neonTeal,
                          ),
                        ),
                      ],
                    );
                  },
                ),
                const SizedBox(height: 24),
                if (tenants.isEmpty)
                  GlassCard(
                    padding: const EdgeInsets.all(24),
                    child: const Text(
                      'No tenants yet. Click "Add Tenant" to create one.',
                      style: TextStyle(color: AppColors.textSecondary),
                    ),
                  )
                else
                  ListView.builder(
                    shrinkWrap: true,
                    physics: const NeverScrollableScrollPhysics(),
                    itemCount: tenants.length,
                    itemBuilder: (context, index) {
                      final tenant = tenants[index];
                      return Padding(
                        padding: const EdgeInsets.only(bottom: 16.0),
                        child: GlassCard(
                          padding: const EdgeInsets.all(24),
                          child: LayoutBuilder(
                            builder: (context, constraints) {
                              final isDesktop = constraints.maxWidth > 600;
                              return Flex(
                                direction: isDesktop
                                    ? Axis.horizontal
                                    : Axis.vertical,
                                crossAxisAlignment: isDesktop
                                    ? CrossAxisAlignment.center
                                    : CrossAxisAlignment.start,
                                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                                children: [
                                  Expanded(
                                    flex: isDesktop ? 1 : 0,
                                    child: Column(
                                      crossAxisAlignment:
                                          CrossAxisAlignment.start,
                                      children: [
                                        Row(
                                          children: [
                                            Text(
                                              tenant.name,
                                              style: const TextStyle(
                                                color: AppColors.textPrimary,
                                                fontSize: 18,
                                                fontWeight: FontWeight.w600,
                                              ),
                                            ),
                                            const SizedBox(width: 12),
                                            Container(
                                              padding:
                                                  const EdgeInsets.symmetric(
                                                horizontal: 8,
                                                vertical: 2,
                                              ),
                                              decoration: BoxDecoration(
                                                color: AppColors.neonTeal
                                                    .withOpacity(0.1),
                                                borderRadius:
                                                    BorderRadius.circular(12),
                                              ),
                                              child: Text(
                                                '${tenant.globalRateCap}/min',
                                                style: const TextStyle(
                                                  color: AppColors.neonTeal,
                                                  fontSize: 12,
                                                  fontWeight: FontWeight.w500,
                                                ),
                                              ),
                                            ),
                                          ],
                                        ),
                                        const SizedBox(height: 16),
                                        Wrap(
                                          spacing: 24,
                                          runSpacing: 16,
                                          children: [
                                            _buildInfoCol(
                                              'API Keys',
                                              tenant.apiKeys.isEmpty
                                                  ? 'None'
                                                  : '${tenant.apiKeys.length} active',
                                            ),
                                            _buildInfoCol(
                                              'Channels',
                                              tenant.enabledChannels.isEmpty
                                                  ? 'None'
                                                  : tenant.enabledChannels
                                                      .join(', '),
                                            ),
                                            _buildInfoCol(
                                              'Created',
                                              tenant.createdAt
                                                  .toLocal()
                                                  .toString()
                                                  .split(' ')[0],
                                            ),
                                          ],
                                        ),
                                      ],
                                    ),
                                  ),
                                  if (!isDesktop) const SizedBox(height: 24),
                                  GestureDetector(
                                    onTap: () => _showSettingsModal(tenant),
                                    child: Container(
                                      padding: const EdgeInsets.symmetric(
                                        horizontal: 16,
                                        vertical: 8,
                                      ),
                                      decoration: BoxDecoration(
                                        color: AppColors.surfaceObsidian
                                            .withOpacity(0.5),
                                        border: Border.all(
                                          color: AppColors.borderSubtle,
                                        ),
                                        borderRadius: BorderRadius.circular(8),
                                      ),
                                      child: const Row(
                                        children: [
                                          Icon(
                                            Icons.settings,
                                            size: 16,
                                            color: AppColors.textPrimary,
                                          ),
                                          SizedBox(width: 8),
                                          Text(
                                            'Settings',
                                            style: TextStyle(
                                              color: AppColors.textPrimary,
                                              fontSize: 14,
                                            ),
                                          ),
                                        ],
                                      ),
                                    ),
                                  ),
                                ],
                              );
                            },
                          ),
                        ),
                      );
                    },
                  ),
              ],
            ),
          ),
        );
      },
    );
  }

  Widget _buildStatCard(String label, String value, Color color) {
    return GlassCard(
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            label,
            style: const TextStyle(
              color: AppColors.textSecondary,
              fontSize: 14,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            value,
            style: TextStyle(
              color: color,
              fontSize: 24,
              fontWeight: FontWeight.bold,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildInfoCol(String label, String value) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          label,
          style: const TextStyle(color: AppColors.textSecondary, fontSize: 14),
        ),
        const SizedBox(height: 4),
        Text(
          value,
          style: const TextStyle(
            color: AppColors.textPrimary,
            fontSize: 14,
            fontWeight: FontWeight.w600,
          ),
        ),
      ],
    );
  }
}

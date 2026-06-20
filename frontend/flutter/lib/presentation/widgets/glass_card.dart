import 'dart:ui';
import 'package:flutter/material.dart';
import '../../core/theme/app_colors.dart';

class GlassCard extends StatefulWidget {
  final Widget child;
  final double blur;
  final double borderRadius;
  final EdgeInsetsGeometry padding;
  final bool animateOnHover;

  const GlassCard({
    super.key,
    required this.child,
    this.blur = 16.0,
    this.borderRadius = 16.0,
    this.padding = const EdgeInsets.all(24.0),
    this.animateOnHover = true,
  });

  @override
  State<GlassCard> createState() => _GlassCardState();
}

class _GlassCardState extends State<GlassCard> {
  bool _isHovered = false;

  @override
  Widget build(BuildContext context) {
    return MouseRegion(
      onEnter: (_) => setState(() => _isHovered = true),
      onExit: (_) => setState(() => _isHovered = false),
      child: AnimatedContainer(
        duration: const Duration(milliseconds: 200),
        curve: Curves.easeOut,
        transform: widget.animateOnHover && _isHovered
            ? (Matrix4.identity()..scale(1.015))
            : Matrix4.identity(),
        child: ClipRRect(
          borderRadius: BorderRadius.circular(widget.borderRadius),
          child: BackdropFilter(
            filter: ImageFilter.blur(sigmaX: widget.blur, sigmaY: widget.blur),
            child: AnimatedContainer(
              duration: const Duration(milliseconds: 200),
              padding: widget.padding,
              decoration: BoxDecoration(
                color: AppColors.surfaceCard.withOpacity(
                  _isHovered ? 0.75 : 0.65,
                ),
                borderRadius: BorderRadius.circular(widget.borderRadius),
                border: Border.all(
                  color: _isHovered
                      ? AppColors.neonTeal.withOpacity(0.25)
                      : Colors.white.withOpacity(0.08),
                  width: 1.0,
                ),
                boxShadow: [
                  BoxShadow(
                    color: _isHovered
                        ? AppColors.neonTeal.withOpacity(0.06)
                        : Colors.black.withOpacity(0.2),
                    blurRadius: _isHovered ? 24.0 : 16.0,
                    offset: const Offset(0, 8),
                  ),
                ],
              ),
              child: widget.child,
            ),
          ),
        ),
      ),
    );
  }
}

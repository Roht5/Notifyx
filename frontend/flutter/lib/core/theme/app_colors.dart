import 'package:flutter/material.dart';

class AppColors {
  // Deep Backgrounds
  static const Color background = Color(0xff0b0f19);
  static const Color surfaceObsidian = Color(0xff111827);
  static const Color surfaceCard = Color(
    0x99111827,
  ); // translucent for glassmorphism

  // Neon Accents
  static const Color neonTeal = Color(0xff00f2fe);
  static const Color electricViolet = Color(0xff4facfe);
  static const Color emeraldGreen = Color(0xff00ff87);
  static const Color coralRed = Color(0xffff4e50);
  static const Color amberWarning = Color(0xffffb300);

  // Text colors
  static const Color textPrimary = Color(0xfff3f4f6);
  static const Color textSecondary = Color(0xff9ca3af);
  static const Color textMuted = Color(0xff6b7280);

  // Border colors
  static const Color borderSubtle = Color(0x14ffffff); // white 8% opacity
  static const Color borderFocus = Color(0x3300f2fe); // neon teal 20% opacity

  // Gradients
  static const LinearGradient primaryGradient = LinearGradient(
    colors: [neonTeal, electricViolet],
    begin: Alignment.topLeft,
    end: Alignment.bottomRight,
  );

  static const LinearGradient successGradient = LinearGradient(
    colors: [emeraldGreen, neonTeal],
    begin: Alignment.topLeft,
    end: Alignment.bottomRight,
  );

  static const LinearGradient alertGradient = LinearGradient(
    colors: [coralRed, electricViolet],
    begin: Alignment.topLeft,
    end: Alignment.bottomRight,
  );
}

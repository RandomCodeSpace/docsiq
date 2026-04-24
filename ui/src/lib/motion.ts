import type { Transition } from "framer-motion";

/**
 * Reduced-motion-aware transition presets.
 *
 * Callers pass the current user preference (from useReducedMotion()) and
 * receive a transition config whose duration collapses to 0 when reduced
 * motion is requested — still completing the state change, just instantly.
 */

export function fadeTransition(reducedMotion: boolean): Transition {
  return {
    duration: reducedMotion ? 0 : 0.18,
    ease: [0.2, 0, 0, 1],
  };
}

export function slideTransition(reducedMotion: boolean): Transition {
  return {
    duration: reducedMotion ? 0 : 0.22,
    ease: [0.2, 0, 0, 1],
  };
}

export function popTransition(reducedMotion: boolean): Transition {
  return {
    duration: reducedMotion ? 0 : 0.16,
    ease: [0.2, 0, 0, 1],
  };
}

/** Variants helper for simple fade-in mounts. */
export const fadeInVariants = {
  hidden: { opacity: 0 },
  visible: { opacity: 1 },
};

/** Variants helper for slide-up mounts (12px travel). */
export const slideUpVariants = {
  hidden: { opacity: 0, y: 12 },
  visible: { opacity: 1, y: 0 },
};

// --- Legacy presets kept for backward compatibility with any existing
// callers in the tree or in parallel blocks. Prefer the factory functions
// above in new code (they participate in reduced-motion gating).
export const enterTransition: Transition = {
  duration: 0.18,
  ease: [0.3, 0, 0, 1],
};

export const exitTransition: Transition = {
  duration: 0.12,
  ease: [0.7, 0, 1, 0.3],
};

export function reducedMotionTransition(): Transition {
  return { duration: 0 };
}

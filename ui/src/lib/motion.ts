import type { Transition } from "framer-motion";

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

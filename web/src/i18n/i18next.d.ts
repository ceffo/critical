import "i18next";

// i18next 27 types `t()` so a string with {{variables}} needs its options. Keys here are plain
// strings built at run time and handed around as a loose `(k, o?) => string`, so keep the looser
// 26 typing.
declare module "i18next" {
  interface CustomTypeOptions {
    strictInterpolationChecks: false;
  }
}

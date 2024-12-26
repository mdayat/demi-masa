import eslint from "@eslint/js";
import tseslint from "typescript-eslint";
import globals from "globals";
import eslintConfigPrettier from "eslint-config-prettier";
import solid from "eslint-plugin-solid/configs/typescript";
import * as tsParser from "@typescript-eslint/parser";

export default tseslint.config(
  { ignores: [".solid"] },
  {
    files: ["**/*.{ts,tsx}"],
    ignores: ["**/*.d.ts"],
    extends: [
      eslint.configs.recommended,
      ...tseslint.configs.recommended,
      eslintConfigPrettier,
    ],
    languageOptions: {
      globals: globals.browser,
      parser: tsParser,
      parserOptions: {
        project: "tsconfig.json",
      },
    },
    plugins: { ...solid.plugins },
    rules: { ...solid.rules },
  }
);

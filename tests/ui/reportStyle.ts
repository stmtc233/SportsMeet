import { readFileSync } from "node:fs";

// The CSS of a report document as Go assembles it (internal/report render.go):
// the base rules, the default parameters' variables and @page, the default
// look. Go's tests pin the parameters' file (testdata/report/params-default.css).
export const reportStyle = ["internal/report/base.css", "testdata/report/params-default.css", "internal/report/default.css"]
  .map((file) => readFileSync(file, "utf8"))
  .join("\n");

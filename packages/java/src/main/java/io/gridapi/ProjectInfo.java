package io.gridapi;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;

/** A project the account key can reach, plus its sheets. */
public record ProjectInfo(String id, String name, String slug, String role, String spreadsheetId,
                          List<SheetInfo> sheets) {
    static ProjectInfo from(Object raw) {
        Map<String, Object> m = Json.asMap(raw);
        List<SheetInfo> sheets = new ArrayList<>();
        for (Object o : Json.asList(m.get("sheets"))) sheets.add(SheetInfo.from(o));
        return new ProjectInfo(
            Json.str(m.get("id")), Json.str(m.get("name")), Json.str(m.get("slug")),
            Json.str(m.get("role")), Json.str(m.get("spreadsheetId")), sheets
        );
    }
}

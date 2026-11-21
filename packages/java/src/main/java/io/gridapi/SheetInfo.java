package io.gridapi;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;

/** One endpoint/sheet exposed by a project. */
public record SheetInfo(String endpoint, String name, String sheetName, String primaryKey,
                        List<String> allowedMethods) {
    static SheetInfo from(Object raw) {
        Map<String, Object> m = Json.asMap(raw);
        List<String> methods = new ArrayList<>();
        for (Object o : Json.asList(m.get("allowedMethods"))) methods.add(String.valueOf(o));
        return new SheetInfo(
            Json.str(m.get("endpoint")), Json.str(m.get("name")),
            Json.str(m.get("sheetName")), Json.str(m.get("primaryKey")), methods
        );
    }
}

package io.gridapi;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;

/** Column definition discovered from a project's docs. */
public record TableSchema(String endpoint, String primaryKey, List<Field> fields) {
    public record Field(String name, String type) {
        static Field from(Object raw) {
            Map<String, Object> m = Json.asMap(raw);
            return new Field(Json.str(m.get("name")), Json.str(m.get("type")));
        }
    }

    static TableSchema from(Object raw) {
        Map<String, Object> m = Json.asMap(raw);
        List<Field> fields = new ArrayList<>();
        for (Object o : Json.asList(m.get("fields"))) fields.add(Field.from(o));
        return new TableSchema(Json.str(m.get("endpoint")), Json.str(m.get("primaryKey")), fields);
    }
}

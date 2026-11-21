package io.gridapi;

import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.Map;

/** A sheet row: column name to string value (null for an empty cell). */
public record GridRecord(Map<String, String> fields) {
    public GridRecord {
        fields = fields == null
            ? Collections.emptyMap()
            : Collections.unmodifiableMap(new LinkedHashMap<>(fields));
    }

    public String get(String column) { return fields.get(column); }

    static GridRecord from(Object raw) {
        Map<String, String> out = new LinkedHashMap<>();
        Json.asMap(raw).forEach((k, v) -> out.put(k, v == null ? null : String.valueOf(v)));
        return new GridRecord(out);
    }
}

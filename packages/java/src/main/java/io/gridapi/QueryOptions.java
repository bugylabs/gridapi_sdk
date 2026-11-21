package io.gridapi;

import java.util.Map;

/** Optional filters and pagination for list/query/count. */
public record QueryOptions(String filter, Integer page, Integer limit) {
    public QueryOptions {
        if (filter != null && filter.isBlank()) filter = null;
    }

    /** Build an exact-match filter string such as "status:active,team:core". */
    public static QueryOptions by(Map<String, String> filter, Integer page, Integer limit) {
        if (filter == null || filter.isEmpty()) return new QueryOptions(null, page, limit);
        StringBuilder sb = new StringBuilder();
        for (Map.Entry<String, String> e : filter.entrySet()) {
            if (sb.length() > 0) sb.append(',');
            sb.append(e.getKey()).append(':').append(e.getValue());
        }
        return new QueryOptions(sb.toString(), page, limit);
    }

    public static QueryOptions of(String filter, Integer page, Integer limit) {
        return new QueryOptions(filter, page, limit);
    }
}

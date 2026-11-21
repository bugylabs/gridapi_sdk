package io.gridapi;

import java.util.Map;

/** Usage counters from the me/usage endpoint; extra server fields stay in raw. */
public record UsageInfo(Map<String, Object> raw) {
    public Long totalApiCallsUsed() { return number("totalApiCallsUsed"); }
    public Long totalApiCallsLeft() { return number("totalApiCallsLeft"); }

    private Long number(String key) {
        Object v = raw.get(key);
        return v instanceof Number n ? n.longValue() : null;
    }

    static UsageInfo from(Object o) { return new UsageInfo(Json.asMap(o)); }
}

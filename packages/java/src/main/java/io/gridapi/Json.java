package io.gridapi;

import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/**
 * Minimal JSON reader/writer. JDK 21 has no standard JSON API, so this stays
 * dependency-free and covers exactly the shapes the GridAPI contract uses.
 */
final class Json {
    private final String src;
    private int pos;

    private Json(String src) { this.src = src; }

    static Object parse(String text) {
        Json j = new Json(text);
        Object value = j.readValue();
        j.ws();
        if (j.pos != j.src.length()) throw j.err("trailing content");
        return value;
    }

    /** Parse, falling back to the raw text when the body is not JSON. */
    static Object tryParse(String text) {
        if (text == null || text.isBlank()) return null;
        try { return parse(text); } catch (RuntimeException e) { return text; }
    }

    @SuppressWarnings("unchecked")
    static Map<String, Object> asMap(Object v) {
        return v instanceof Map<?, ?> m ? (Map<String, Object>) m : Map.of();
    }

    @SuppressWarnings("unchecked")
    static List<Object> asList(Object v) {
        return v instanceof List<?> l ? (List<Object>) l : List.of();
    }

    static String str(Object v) { return v instanceof String s ? s : null; }

    static int intOf(Object v, int fallback) {
        if (v instanceof Number n) return n.intValue();
        if (v instanceof String s) {
            try { return Integer.parseInt(s); } catch (NumberFormatException ignored) { }
        }
        return fallback;
    }

    private void ws() {
        while (pos < src.length() && Character.isWhitespace(src.charAt(pos))) pos++;
    }

    private Object readValue() {
        ws();
        if (pos >= src.length()) throw err("unexpected end");
        return switch (src.charAt(pos)) {
            case '{' -> object();
            case '[' -> array();
            case '"' -> string();
            case 't' -> literal("true", Boolean.TRUE);
            case 'f' -> literal("false", Boolean.FALSE);
            case 'n' -> literal("null", null);
            default -> number();
        };
    }

    private Object literal(String word, Object value) {
        if (!src.startsWith(word, pos)) throw err("invalid literal");
        pos += word.length();
        return value;
    }

    private Map<String, Object> object() {
        pos++;
        Map<String, Object> out = new LinkedHashMap<>();
        ws();
        if (peek('}')) { pos++; return out; }
        while (true) {
            ws();
            String key = string();
            ws();
            if (!peek(':')) throw err("expected ':'");
            pos++;
            out.put(key, readValue());
            ws();
            if (peek(',')) { pos++; continue; }
            if (peek('}')) { pos++; return out; }
            throw err("expected ',' or '}'");
        }
    }

    private List<Object> array() {
        pos++;
        List<Object> out = new ArrayList<>();
        ws();
        if (peek(']')) { pos++; return out; }
        while (true) {
            out.add(readValue());
            ws();
            if (peek(',')) { pos++; continue; }
            if (peek(']')) { pos++; return out; }
            throw err("expected ',' or ']'");
        }
    }

    private String string() {
        ws();
        if (!peek('"')) throw err("expected string");
        pos++;
        StringBuilder sb = new StringBuilder();
        while (pos < src.length()) {
            char c = src.charAt(pos++);
            if (c == '"') return sb.toString();
            if (c != '\\') { sb.append(c); continue; }
            if (pos >= src.length()) throw err("bad escape");
            char e = src.charAt(pos++);
            switch (e) {
                case '"' -> sb.append('"');
                case '\\' -> sb.append('\\');
                case '/' -> sb.append('/');
                case 'b' -> sb.append('\b');
                case 'f' -> sb.append('\f');
                case 'n' -> sb.append('\n');
                case 'r' -> sb.append('\r');
                case 't' -> sb.append('\t');
                case 'u' -> {
                    if (pos + 4 > src.length()) throw err("bad unicode escape");
                    sb.append((char) Integer.parseInt(src.substring(pos, pos + 4), 16));
                    pos += 4;
                }
                default -> throw err("bad escape");
            }
        }
        throw err("unterminated string");
    }

    private Object number() {
        int start = pos;
        if (peek('-')) pos++;
        boolean floating = false;
        while (pos < src.length()) {
            char c = src.charAt(pos);
            if (Character.isDigit(c)) { pos++; continue; }
            if (c == '.' || c == 'e' || c == 'E' || c == '+' || c == '-') {
                floating = floating || c == '.' || c == 'e' || c == 'E';
                pos++;
                continue;
            }
            break;
        }
        String token = src.substring(start, pos);
        if (token.isEmpty()) throw err("expected number");
        try {
            return floating ? Double.valueOf(token) : Long.valueOf(token);
        } catch (NumberFormatException e) {
            throw err("bad number: " + token);
        }
    }

    private boolean peek(char c) { return pos < src.length() && src.charAt(pos) == c; }
    private IllegalArgumentException err(String msg) {
        return new IllegalArgumentException("JSON at " + pos + ": " + msg);
    }

    static String write(Object v) {
        StringBuilder sb = new StringBuilder();
        emit(v, sb);
        return sb.toString();
    }

    private static void emit(Object v, StringBuilder sb) {
        if (v == null) { sb.append("null"); return; }
        if (v instanceof String s) { quote(s, sb); return; }
        if (v instanceof Boolean || v instanceof Number) { sb.append(v); return; }
        if (v instanceof Map<?, ?> m) {
            sb.append('{');
            boolean first = true;
            for (Map.Entry<?, ?> e : m.entrySet()) {
                if (!first) sb.append(',');
                first = false;
                quote(String.valueOf(e.getKey()), sb);
                sb.append(':');
                emit(e.getValue(), sb);
            }
            sb.append('}');
            return;
        }
        if (v instanceof Iterable<?> it) {
            sb.append('[');
            boolean first = true;
            for (Object o : it) {
                if (!first) sb.append(',');
                first = false;
                emit(o, sb);
            }
            sb.append(']');
            return;
        }
        throw new IllegalArgumentException("Cannot JSON-encode " + v.getClass());
    }

    private static void quote(String s, StringBuilder sb) {
        sb.append('"');
        for (int i = 0; i < s.length(); i++) {
            char c = s.charAt(i);
            switch (c) {
                case '"' -> sb.append("\\\"");
                case '\\' -> sb.append("\\\\");
                case '\n' -> sb.append("\\n");
                case '\r' -> sb.append("\\r");
                case '\t' -> sb.append("\\t");
                case '\b' -> sb.append("\\b");
                case '\f' -> sb.append("\\f");
                default -> {
                    if (c < 0x20) sb.append(String.format("\\u%04x", (int) c));
                    else sb.append(c);
                }
            }
        }
        sb.append('"');
    }
}

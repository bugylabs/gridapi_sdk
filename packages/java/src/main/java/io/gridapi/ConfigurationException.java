package io.gridapi;

/** Thrown at construction time when a required key is missing or malformed. */
public class ConfigurationException extends GridApiError {
    public ConfigurationException(String message) { super(message); }
    public ConfigurationException(String message, Options o) { super(message, o); }
}

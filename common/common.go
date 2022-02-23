package common

import (
	"fmt"

	"golang.org/x/sys/windows/registry"
)

func RegistrySaveString(subKeyPath string, valueName string, value string) error {
	// walk doesn't provide registry set/save functions. Nor even create key. So use the windows
	// package for that.
	key, _, err := registry.CreateKey(registry.CURRENT_USER, subKeyPath, registry.ALL_ACCESS)
	if err != nil {
		return fmt.Errorf("%v: CreateKey() failed: %w", subKeyPath, err)
	}

	defer key.Close()

	err = key.SetStringValue(valueName, value)
	if err != nil {
		return fmt.Errorf("%v: RegSetValueEx(%v) failed: %w", subKeyPath, valueName, err)
	}

	return nil
}

func RegistryLoadString(subKeyPath string, valueName string) (string, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, subKeyPath, registry.READ)
	if err != nil {
		return "", fmt.Errorf("%v: OpenKey() failed: %w", subKeyPath, err)
	}

	defer key.Close()

	val, valType, err := key.GetStringValue(valueName)
	if err != nil {
		return "", fmt.Errorf("%v: GetStringValue(%v) failed: %w", subKeyPath, valueName, err)
	}

	if valType != registry.SZ {
		return "", fmt.Errorf("%v: GetStringValue(%v) returned invalid type %v", subKeyPath, valueName, valType)
	}

	return val, nil
}

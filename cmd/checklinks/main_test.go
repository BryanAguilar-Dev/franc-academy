package main

import (
	"errors"
	"testing"
)

// Sin internet: comprueba que se leen las lecciones reales y que todas las
// URLs se recogen, una sola vez cada una.
func TestRecogerURLs(t *testing.T) {
	usos, err := recogerURLs("../../content/*/*.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(usos) == 0 {
		t.Fatal("no se ha recogido ninguna URL")
	}
	for u, archivos := range usos {
		if u == "" || len(archivos) == 0 {
			t.Errorf("URL vacía o sin lección: %q %v", u, archivos)
		}
	}
}

func TestClasificar(t *testing.T) {
	casos := []struct {
		r    resultado
		want string
	}{
		{resultado{estado: 200}, "ok"},
		{resultado{estado: 403}, "aviso"},
		{resultado{estado: 429}, "aviso"},
		{resultado{estado: 404}, "roto"},
		{resultado{estado: 503}, "roto"},
		{resultado{err: errors.New("timeout")}, "roto"},
	}
	for _, c := range casos {
		if got := clasificar(c.r); got != c.want {
			t.Errorf("clasificar(%+v) = %s, quiero %s", c.r, got, c.want)
		}
	}
}

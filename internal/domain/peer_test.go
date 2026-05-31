package domain

import (
	"net"
	"testing"
)

func TestNewPeer(t *testing.T) {
	t.Run("Deve criar um Peer válido", func(t *testing.T){
		peer, err := NewPeer("client-1", "iPhone", "based-pub-key")
		if err != nil {
			t.Fatalf("esperava sucesso, recebeu erro: %v", err)
		}
		if peer.IsRevoked {
			t.Errorf("um peer novo não deve nascer revogado")
		}
	})

	t.Run("Deve falhar ao criar Peer sem dados obrigatórios", func(t *testing.T){
		_, err := NewPeer("client-2", "", "")
		if err != ErrInvalidPeerData {
			t.Errorf("esperava ErrInvalidPeerData, recebeu: %v", err)
		}
	})
}

func TestPeer_AssignIP(t *testing.T) {
	peer, _ := NewPeer("client-1", "iPhone", "key")
	ip := net.ParseIP("10.8.0.2")

	t.Run("Deve atribuir IP com sucesso", func(t *testing.T) {
		err := peer.AssignIP(ip)
		
		if err != nil {
			t.Fatalf("nao esperava erro: %v", err)
		}
		if !peer.AllocatedIP.Equal(ip) {
			t.Errorf("IP atribuido incorretamente. Esperado: %s, Obtido: %s", ip, peer.AllocatedIP)
		}
	})

	t.Run("Nao deve atribuir IP se estiver revogado", func(t *testing.T) {
		peer.Revoke()
		err := peer.AssignIP(net.ParseIP("10.8.0.3"))
		
		if err != ErrPeerRevoked {
			t.Errorf("esperava ErrPeerRevoked, recebeu: %v", err)
		}
	})
}
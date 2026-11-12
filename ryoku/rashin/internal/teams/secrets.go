package teams

import (
	i18n "ryoku-i18n"
	"ryoku-rashin/internal/protocol"
)

// Secret returns the value of the vault credential called name, for a memory
// engine's key. It never reaches a client or a log.
func (s *Service) Secret(name string) (string, error) {
	c, ok := s.vault.Find(name)
	if !ok {
		return "", protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("The vault holds no credential named %s.", name))
	}
	return s.vault.Reveal(c.ID)
}

// Keep seals a value Rashin made for itself, such as the key of a memory
// engine it runs, under name. A credential that already has the name gets
// the value as its new version. It goes only to this computer and to no
// team.
func (s *Service) Keep(name, value string) error {
	if c, ok := s.vault.Find(name); ok {
		updated, err := s.vault.Update(protocol.VaultUpdateParams{ID: c.ID, Value: value})
		if err != nil {
			return err
		}
		s.recordCredential(protocol.VaultEvent{Credential: updated.ID, Name: updated.Name, Outcome: protocol.VaultRotated, Detail: i18n.T("Rashin replaced its value for its memory engine.")})
		s.emitVault()
		return nil
	}
	c, err := s.vault.Add(protocol.VaultAddParams{
		Name: name, Description: i18n.T("Made by Rashin for its memory engine."),
		Use: protocol.CredentialHeader, Prefix: "Bearer ", Hosts: []string{"127.0.0.1"}, Value: value,
	})
	if err != nil {
		return err
	}
	s.recordCredential(protocol.VaultEvent{Credential: c.ID, Name: c.Name, Outcome: protocol.VaultAdded, Detail: i18n.T("Rashin made it for its memory engine.")})
	s.emitVault()
	return nil
}

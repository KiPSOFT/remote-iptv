import React, { useState } from 'react';
import {
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  TextField,
  Button,
  Box,
} from '@mui/material';

interface XtreamSettings {
  url: string;
  username: string;
  password: string;
}

interface Props {
  open: boolean;
  onClose: () => void;
  onSave: (settings: XtreamSettings) => void;
  initialSettings?: XtreamSettings;
}

export default function XtreamSettingsDialog({ open, onClose, onSave, initialSettings }: Props) {
  const [settings, setSettings] = useState<XtreamSettings>(
    initialSettings || {
      url: '',
      username: '',
      password: '',
    }
  );

  const handleSave = () => {
    onSave(settings);
    onClose();
  };

  return (
    <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>Xtream Ayarları</DialogTitle>
      <DialogContent>
        <Box sx={{ pt: 2 }}>
          <TextField
            fullWidth
            label="Sunucu URL"
            value={settings.url}
            onChange={(e) => setSettings({ ...settings, url: e.target.value })}
            margin="normal"
            placeholder="http://example.com:8080"
          />
          <TextField
            fullWidth
            label="Kullanıcı Adı"
            value={settings.username}
            onChange={(e) => setSettings({ ...settings, username: e.target.value })}
            margin="normal"
          />
          <TextField
            fullWidth
            label="Şifre"
            type="password"
            value={settings.password}
            onChange={(e) => setSettings({ ...settings, password: e.target.value })}
            margin="normal"
          />
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>İptal</Button>
        <Button
          onClick={handleSave}
          variant="contained"
          disabled={!settings.url || !settings.username || !settings.password}
        >
          Kaydet
        </Button>
      </DialogActions>
    </Dialog>
  );
} 
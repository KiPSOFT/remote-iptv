import React from 'react';
import { 
  AppBar, 
  Toolbar, 
  Typography, 
  Box, 
  TextField,
  InputAdornment,
  IconButton
} from '@mui/material';
import { Search, Stop } from '@mui/icons-material';
import type { Channel } from '../types';

interface HeaderProps {
  searchQuery: string;
  onSearchChange: (query: string) => void;
  currentChannel: Channel | null;
  mpvStatus: boolean;
  onStopChannel: () => void;
}

const Header: React.FC<HeaderProps> = ({ 
  searchQuery, 
  onSearchChange, 
  currentChannel,
  mpvStatus,
  onStopChannel
}) => {
  return (
    <AppBar position="sticky">
      <Toolbar>
        <Typography variant="h6" component="div" sx={{ flexGrow: 1 }}>
          IPTV Player
          {mpvStatus && currentChannel && (
            <Typography variant="subtitle1" component="span" sx={{ ml: 2, fontSize: '0.9rem' }}>
              Oynatılıyor: {currentChannel.name}
              <IconButton
                size="small"
                onClick={onStopChannel}
                sx={{ ml: 1, color: 'inherit' }}
              >
                <Stop />
              </IconButton>
            </Typography>
          )}
        </Typography>
        <Box sx={{ display: 'flex', alignItems: 'center' }}>
          <TextField
            size="small"
            placeholder="Kanal ara..."
            value={searchQuery}
            onChange={(e) => onSearchChange(e.target.value)}
            InputProps={{
              startAdornment: (
                <InputAdornment position="start">
                  <Search />
                </InputAdornment>
              ),
              sx: { 
                bgcolor: 'background.paper',
                borderRadius: 1
              }
            }}
            sx={{ mr: 2 }}
          />
        </Box>
      </Toolbar>
    </AppBar>
  );
}; 
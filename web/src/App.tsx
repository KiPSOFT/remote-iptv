import React, { useState, useEffect, useRef, useCallback } from 'react';
import {
  AppBar,
  Box,
  CssBaseline,
  Drawer,
  IconButton,
  List,
  ListItem,
  ListItemButton,
  ListItemIcon,
  ListItemText,
  Toolbar,
  Typography,
  Container,
  Card,
  CardContent,
  CardActions,
  Button,
  Alert,
  CircularProgress,
  Collapse,
  Grid,
  TextField,
  InputAdornment,
  Menu,
  MenuItem,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogContentText,
  DialogActions,
  Link,
  useTheme,
} from '@mui/material';
import {
  Menu as MenuIcon,
  Favorite,
  PlayArrow,
  Stop,
  Star,
  StarBorder,
  Settings as SettingsIcon,
  LiveTv,
  Movie,
  Theaters,
  ExpandLess,
  ExpandMore,
  Refresh as RefreshIcon,
  Search as SearchIcon,
  Sort as SortIcon,
  DarkMode as DarkModeIcon,
  LightMode as LightModeIcon,
  Info as InfoIcon,
} from '@mui/icons-material';
import axios from 'axios';
import XtreamSettingsDialog from './components/XtreamSettingsDialog';

const drawerWidth = 240;

interface Channel {
  id: number;
  name: string;
  url: string;
  stream_type: string;
  category_id: number;
  stream_icon?: string;
  rating?: number;
  added?: string;
}

interface Category {
  category_id: number;
  category_name: string;
  parent_id?: number;
  type: string;
}

interface XtreamSettings {
  url: string;
  username: string;
  password: string;
}

function App() {
  const [mobileOpen, setMobileOpen] = useState(false);
  const [channels, setChannels] = useState<Channel[]>([]);
  const [categories, setCategories] = useState<Category[]>([]);
  const [favorites, setFavorites] = useState<Channel[]>([]);
  const [currentChannel, setCurrentChannel] = useState<Channel | null>(null);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [xtreamSettings, setXtreamSettings] = useState<XtreamSettings | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [selectedCategory, setSelectedCategory] = useState<string>('');
  const [expandedCategories, setExpandedCategories] = useState<{ [key: string]: boolean }>({
    live: true,
    movie: false,
    series: false,
  });
  const [searchQuery, setSearchQuery] = useState('');
  const [filteredChannels, setFilteredChannels] = useState<Channel[]>([]);
  const [allChannels, setAllChannels] = useState<Channel[]>([]);
  const [sortAnchorEl, setSortAnchorEl] = useState<null | HTMLElement>(null);
  const [sortOrder, setSortOrder] = useState<'newest' | 'oldest' | 'rating'>('newest');
  const [mpvStatus, setMpvStatus] = useState<boolean>(false);
  const [isLoading, setIsLoading] = useState(false);
  const [darkMode, setDarkMode] = useState(localStorage.getItem('darkMode') === 'true');
  const [aboutDialogOpen, setAboutDialogOpen] = useState(false);
  const theme = useTheme();

  const checkMpvStatus = useCallback(async () => {
    try {
      const response = await axios.get('/api/player/status');
      
      console.log('MPV status:', response.data);
      
      const isRunning = response.data.isRunning || false;
      
      // Eğer MPV durumu değiştiyse UI'ı güncelle
      if (isRunning !== mpvStatus) {
        setMpvStatus(isRunning);
        
        // Eğer MPV durdu ancak UI'da hala çalışıyor görünüyorsa, UI'ı güncelle
        if (!isRunning && mpvStatus) {
          setCurrentChannel(null);
        } else if (isRunning && response.data.currentChannel) {
          setCurrentChannel(response.data.currentChannel);
        }
      }
    } catch (error) {
      console.error('Error checking MPV status:', error);
      setMpvStatus(false);
      setCurrentChannel(null);
    }
  }, [mpvStatus]);

  useEffect(() => {
    const fetchSettings = async () => {
      try {
        const response = await axios.get('/api/xtream/settings');
        console.log('Xtream settings response:', response.data);
        if (response.data && (typeof response.data === 'object' && (response.data.url || response.data.username || response.data.password))) {
          setXtreamSettings(response.data);
        } else {
          console.log('Xtream settings empty or invalid, opening settings dialog');
          setSettingsOpen(true);
        }
      } catch (error: any) {
        console.error('Error fetching xtream settings:', error);
        if (error.response?.status === 404) {
          setSettingsOpen(true);
        }
      }
    };

    fetchSettings();
    fetchFavorites();
    
    // İlk başta MPV durumunu kontrol et
    checkMpvStatus();
    
    // Her 3 saniyede bir MPV durumunu kontrol et
    const statusInterval = setInterval(checkMpvStatus, 3000);
    
    // Komponent unmount olduğunda interval'i temizle
    return () => {
      clearInterval(statusInterval);
    };
  }, [checkMpvStatus]);

  useEffect(() => {
    if (xtreamSettings) {
      fetchCategories();
    }
  }, [xtreamSettings]);

  useEffect(() => {
    fetchAllChannels();
  }, []);

  useEffect(() => {
    if (searchQuery.trim() === '') {
      // Arama temizlendiğinde, eğer bir kategori seçiliyse o kategorinin kanallarını göster
      // Aksi halde boş bir liste göster
      if (selectedCategory) {
        setFilteredChannels(channels);
        setChannels(channels); // Filtrelenmiş kanalları sıfırla
      } else {
        setFilteredChannels([]);
      }
    } else if (searchQuery.trim().length >= 2) {
      // Bütün kanallar içinde ara
      const filtered = allChannels.filter(channel =>
        channel.name.toLowerCase().includes(searchQuery.toLowerCase())
      );
      setFilteredChannels(filtered);
      
      // Arama sonuçlarını görüntüle
      if (filtered.length > 0) {
        setChannels(filtered);
      }
    } else {
      setFilteredChannels(channels);
    }
  }, [searchQuery, channels, allChannels, selectedCategory]);

  const fetchCategories = async () => {
    try {
      setLoading(true);
      const [liveResponse, movieResponse, seriesResponse] = await Promise.all([
        axios.get('/api/categories/live'),
        axios.get('/api/categories/movie'),
        axios.get('/api/categories/series'),
      ]);
      
      const allCategories = [
        ...liveResponse.data.map((cat: any) => ({ ...cat, type: 'live' })),
        ...movieResponse.data.map((cat: any) => ({ ...cat, type: 'movie' })),
        ...seriesResponse.data.map((cat: any) => ({ ...cat, type: 'series' })),
      ];
      
      setCategories(allCategories);
    } catch (error) {
      setError('Kategoriler yüklenemedi.');
    } finally {
      setLoading(false);
    }
  };

  const handleCategoryExpand = (category: string) => {
    setExpandedCategories(prev => ({
      ...prev,
      [category]: !prev[category],
    }));
  };

  const filterChannelsByType = (type: string) => {
    switch (type) {
      case 'live':
        return channels.filter(ch => ch.stream_type === 'live');
      case 'movie':
        return channels.filter(ch => ch.stream_type === 'movie');
      case 'series':
        return channels.filter(ch => ch.stream_type === 'series');
      default:
        return [];
    }
  };

  const getCategoriesByType = (type: string) => {
    // Kategorileri alfabetik olarak sırala
    return categories
      .filter(cat => cat.type === type)
      .sort((a, b) => a.category_name.localeCompare(b.category_name));
  };

  const getCategoryType = (categoryId: number): string => {
    const category = categories.find(cat => cat.category_id === categoryId);
    return category?.type || 'live';
  };

  const handleSaveSettings = async (settings: XtreamSettings) => {
    try {
      await axios.post('/api/xtream/settings', settings);
      setXtreamSettings(settings);
      setError(null);
      handleUpdateChannels();
    } catch (error) {
      setError('Xtream ayarları kaydedilemedi.');
    }
  };

  const fetchChannelsByType = async (type: string) => {
    try {
      setLoading(true);
      setError(null);
      const response = await axios.get(`/api/channels/${type}`);
      setChannels(response.data);
    } catch (error: any) {
      setError('Kanallar yüklenirken bir hata oluştu.');
      setChannels([]);
    } finally {
      setLoading(false);
    }
  };

  const handleCategoryClick = async (type: string, categoryId: number) => {
    try {
      setSelectedCategory(`${type}-${categoryId}`);
      setLoading(true);
      setError(null);
      const response = await axios.get(`/api/channels/${type}/${categoryId}`);
      setChannels(response.data);

      // Film ve dizi kategorileri için varsayılan sıralama
      if (type === 'movie' || type === 'series') {
        setSortOrder('newest');
      }
    } catch (error: any) {
      setError('Kanallar yüklenirken bir hata oluştu.');
      setChannels([]);
    } finally {
      setLoading(false);
    }
  };

  const fetchFavorites = async () => {
    try {
      const response = await axios.get('/api/favorites');
      console.log('Favorites response:', response.data);
      setFavorites(response.data || []);
    } catch (error) {
      console.error('Error fetching favorites:', error);
      setFavorites([]);
    }
  };

  const handlePlayChannel = async (channelData: Channel) => {
    setError(null);
    
    if (!channelData.url) {
      console.error("Channel URL is missing");
      setError("Channel URL is missing");
      return;
    }
    
    try {
      setIsLoading(true);
      
      // Stream tipini belirle
      let streamType = channelData.stream_type || 'live';
      
      // URL'ye göre içerik tipini kontrol et ve doğru stream tipini seç
      if (!channelData.stream_type) {
        const urlLower = channelData.url.toLowerCase();
        if (urlLower.includes('/movie/') || urlLower.includes('/vod/') || 
            urlLower.includes('film') || urlLower.includes('movie')) {
          streamType = 'movie';
          console.log("Detected content as movie based on URL");
        } else if (urlLower.includes('/series/') || urlLower.includes('dizi')) {
          streamType = 'series';
          console.log("Detected content as series based on URL");
        }
      }
      
      // Debug: Kanal bilgilerini logla
      console.log("Playing channel:", {
        ...channelData,
        stream_type: streamType
      });
      
      const requestData = {
        url: channelData.url,
        name: channelData.name,
        id: channelData.id || 0,  // ID yoksa 0 gönder
        stream_type: streamType
      };
      
      console.log("Sending request data:", requestData);
      
      try {
        const response = await axios.post('/api/player/play', requestData);
        console.log("Play response:", response.data);
        
        // Anlık kanal güncelleme
        setCurrentChannel(channelData);
        setMpvStatus(true);
        
        // MPV durumunu 1 saniye sonra kontrol et
        setTimeout(checkMpvStatus, 1000);
      } catch (error) {
        console.error("API error playing channel:", error);
        let errorMessage = "Kanal oynatılırken bir hata oluştu.";
        
        if (axios.isAxiosError(error) && error.response) {
          errorMessage = `API Hatası: ${error.response.data || error.message}`;
        }
        
        setError(errorMessage);
        setMpvStatus(false);
      }
      
    } catch (error) {
      console.error("Error playing channel:", error);
      setError(`Hata: ${error instanceof Error ? error.message : String(error)}`);
      setMpvStatus(false);
    } finally {
      setIsLoading(false);
    }
  };

  const handleStopChannel = async () => {
    try {
      setError(null);
      console.log('Stopping channel...');
      
      // Durdurma isteği gönder
      await axios.post('/api/player/stop');
      
      // MPV durumunun hemen güncellenmesi için değerleri doğrudan ayarla
      setCurrentChannel(null);
      setMpvStatus(false);
      
      // Periyodik kontrol zaten var ve durumu güncelleyecek
      
    } catch (error: any) {
      if (error.response?.status === 404) {
        // Player zaten kapalıysa state'i güncelle
        setCurrentChannel(null);
        setMpvStatus(false);
      } else {
        console.error('Error stopping channel:', error);
        setError('Kanal durdurulurken bir hata oluştu');
      }
    }
  };

  const handleToggleFavorite = async (channel: Channel) => {
    try {
      console.log('Adding favorite:', channel);
      await axios.post('/api/favorites', channel);
      console.log('Successfully added favorite');
      await fetchFavorites();
    } catch (error) {
      console.error('Error toggling favorite:', error);
    }
  };

  const handleDrawerToggle = () => {
    setMobileOpen(!mobileOpen);
  };

  const handleUpdateChannels = async () => {
    try {
      setLoading(true);
      await axios.post('/api/xtream/update');
      // Tüm kategorileri yeniden yükle
      await fetchCategories();
      // Tüm kanalları yeniden yükle
      await fetchAllChannels();
      // Mevcut kategorideki kanalları yeniden yükle
      if (selectedCategory) {
        const [type, categoryId] = selectedCategory.split('-');
        if (categoryId) {
          await handleCategoryClick(type, parseInt(categoryId));
        } else if (type === 'favorites') {
          await fetchFavorites();
        }
      }
      setError(null);
    } catch (error: any) {
      if (error.response?.data?.error === 'xtream_settings_required') {
        setSettingsOpen(true);
      } else {
        setError('Kanallar güncellenirken bir hata oluştu.');
      }
    } finally {
      setLoading(false);
    }
  };

  const fetchAllChannels = async () => {
    try {
      const response = await axios.get('/api/channels');
      setAllChannels(response.data);
    } catch (error) {
      console.error('Error fetching all channels:', error);
    }
  };

  const handleFavoritesClick = async () => {
    try {
      setSelectedCategory('favorites');
      setLoading(true);
      const response = await axios.get('/api/favorites');
      console.log('Fetched favorites:', response.data);
      setChannels(response.data || []);
    } catch (error) {
      console.error('Error fetching favorites:', error);
      setError('Favoriler yüklenirken bir hata oluştu.');
      setChannels([]);
    } finally {
      setLoading(false);
    }
  };

  const drawer = (
    <div>
      <Toolbar />
      <List>
        <ListItem disablePadding>
          <ListItemButton onClick={handleUpdateChannels} disabled={loading}>
            <ListItemIcon>
              <RefreshIcon />
            </ListItemIcon>
            <ListItemText primary={loading ? "Güncelleniyor..." : "Kanalları Güncelle"} />
          </ListItemButton>
        </ListItem>

        <ListItem disablePadding>
          <ListItemButton 
            onClick={handleFavoritesClick}
            selected={selectedCategory === 'favorites'}
          >
            <ListItemIcon>
              <Favorite />
            </ListItemIcon>
            <ListItemText primary="Favoriler" />
          </ListItemButton>
        </ListItem>

        {/* Live TV */}
        <ListItem disablePadding>
          <ListItemButton onClick={() => handleCategoryExpand('live')}>
            <ListItemIcon>
              <LiveTv />
            </ListItemIcon>
            <ListItemText primary="Live TV" />
            {expandedCategories.live ? <ExpandLess /> : <ExpandMore />}
          </ListItemButton>
        </ListItem>
        <Collapse in={expandedCategories.live} timeout="auto" unmountOnExit>
          <List component="div" disablePadding>
            {getCategoriesByType('live').map((category) => (
              <ListItem key={category.category_id} disablePadding>
                <ListItemButton 
                  sx={{ pl: 4 }}
                  onClick={() => handleCategoryClick('live', category.category_id)}
                  selected={selectedCategory === `live-${category.category_id}`}
                >
                  <ListItemText primary={category.category_name} />
                </ListItemButton>
              </ListItem>
            ))}
          </List>
        </Collapse>

        {/* Movies */}
        <ListItem disablePadding>
          <ListItemButton onClick={() => handleCategoryExpand('movie')}>
            <ListItemIcon>
              <Movie />
            </ListItemIcon>
            <ListItemText primary="Movies" />
            {expandedCategories.movie ? <ExpandLess /> : <ExpandMore />}
          </ListItemButton>
        </ListItem>
        <Collapse in={expandedCategories.movie} timeout="auto" unmountOnExit>
          <List component="div" disablePadding>
            {getCategoriesByType('movie').map((category) => (
              <ListItem key={category.category_id} disablePadding>
                <ListItemButton 
                  sx={{ pl: 4 }}
                  onClick={() => handleCategoryClick('movie', category.category_id)}
                  selected={selectedCategory === `movie-${category.category_id}`}
                >
                  <ListItemText primary={category.category_name} />
                </ListItemButton>
              </ListItem>
            ))}
          </List>
        </Collapse>

        {/* Series */}
        <ListItem disablePadding>
          <ListItemButton onClick={() => handleCategoryExpand('series')}>
            <ListItemIcon>
              <Theaters />
            </ListItemIcon>
            <ListItemText primary="Series" />
            {expandedCategories.series ? <ExpandLess /> : <ExpandMore />}
          </ListItemButton>
        </ListItem>
        <Collapse in={expandedCategories.series} timeout="auto" unmountOnExit>
          <List component="div" disablePadding>
            {getCategoriesByType('series').map((category) => (
              <ListItem key={category.category_id} disablePadding>
                <ListItemButton 
                  sx={{ pl: 4 }}
                  onClick={() => handleCategoryClick('series', category.category_id)}
                  selected={selectedCategory === `series-${category.category_id}`}
                >
                  <ListItemText primary={category.category_name} />
                </ListItemButton>
              </ListItem>
            ))}
          </List>
        </Collapse>

        <ListItem disablePadding>
          <ListItemButton onClick={() => setSettingsOpen(true)}>
            <ListItemIcon>
              <SettingsIcon />
            </ListItemIcon>
            <ListItemText primary="Ayarlar" />
          </ListItemButton>
        </ListItem>
      </List>
    </div>
  );

  const LazyImage = ({ src, alt }: { src: string | undefined, alt: string }) => {
    const imgRef = useRef<HTMLImageElement>(null);
    const [isLoaded, setIsLoaded] = useState(false);

    useEffect(() => {
      const observer = new IntersectionObserver(
        (entries) => {
          entries.forEach((entry) => {
            if (entry.isIntersecting && imgRef.current && !isLoaded) {
              imgRef.current.src = src || '';
              setIsLoaded(true);
            }
          });
        },
        {
          rootMargin: '50px',
          threshold: 0.1
        }
      );

      if (imgRef.current) {
        observer.observe(imgRef.current);
      }

      return () => {
        if (imgRef.current) {
          observer.unobserve(imgRef.current);
        }
      };
    }, [src, isLoaded]);

    return (
      <img
        ref={imgRef}
        alt={alt}
        style={{ width: '40px', height: '40px', objectFit: 'contain' }}
        src={isLoaded ? src : 'data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7'}
      />
    );
  };

  const handleSortClick = (event: React.MouseEvent<HTMLButtonElement>) => {
    setSortAnchorEl(event.currentTarget);
  };

  const handleSortClose = () => {
    setSortAnchorEl(null);
  };

  const sortChannels = (channelsToSort: Channel[]): Channel[] => {
    if (!channelsToSort) return [];
    
    // Live kanalları için sıralama yapmıyoruz
    const isLiveCategory = selectedCategory?.startsWith('live') || 
                          (selectedCategory === 'favorites' && channelsToSort.some(c => c.stream_type === 'live'));
    
    if (isLiveCategory) {
      return channelsToSort;
    }
    
    return [...channelsToSort].sort((a, b) => {
      switch (sortOrder) {
        case 'newest':
          // Eğer added alanı yoksa, id'yi kullan (daha yüksek id daha yeni)
          if (!a.added && !b.added) {
            return b.id - a.id;
          }
          return (b.added ? new Date(b.added).getTime() : 0) - (a.added ? new Date(a.added).getTime() : 0);
        case 'oldest':
          if (!a.added && !b.added) {
            return a.id - b.id;
          }
          return (a.added ? new Date(a.added).getTime() : 0) - (b.added ? new Date(b.added).getTime() : 0);
        case 'rating':
          const ratingA = typeof a.rating === 'string' ? parseFloat(a.rating) : (a.rating || 0);
          const ratingB = typeof b.rating === 'string' ? parseFloat(b.rating) : (b.rating || 0);
          return ratingB - ratingA;
        default:
          return 0;
      }
    });
  };

  const renderChannelList = () => {
    if (loading) {
      return (
        <Box display="flex" justifyContent="center" p={3}>
          <CircularProgress />
        </Box>
      );
    }

    // Arama yapılıyorsa ve henüz bir kategori seçilmemişse
    if (searchQuery.trim().length >= 2 && !selectedCategory) {
      const filteredResults = allChannels.filter(channel => 
        channel.name.toLowerCase().includes(searchQuery.toLowerCase())
      );
      
      if (filteredResults.length === 0) {
        return (
          <Alert severity="info">
            Arama sonucu bulunamadı.
          </Alert>
        );
      }
      
      return (
        <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(200px, 1fr))', gap: 2, p: 2 }}>
          {filteredResults.map((channel) => (
            // channel kartı
            <Card key={channel.id} sx={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
              <CardContent sx={{ flexGrow: 1, display: 'flex', flexDirection: 'column' }}>
                {channel.stream_icon && (
                  <Box
                    component="img"
                    src={channel.stream_icon}
                    alt={channel.name}
                    sx={{
                      width: '100%',
                      height: 120,
                      objectFit: 'contain',
                      mb: 1,
                      loading: 'lazy'
                    }}
                    loading="lazy"
                  />
                )}
                <Typography
                  gutterBottom
                  variant="subtitle1"
                  component="div"
                  sx={{
                    overflow: 'hidden',
                    textOverflow: 'ellipsis',
                    display: '-webkit-box',
                    WebkitLineClamp: 2,
                    WebkitBoxOrient: 'vertical',
                    minHeight: '3em'
                  }}
                >
                  {channel.name}
                </Typography>
                {channel.rating && (
                  <Typography variant="body2" color="text.secondary">
                    IMDB: {channel.rating}
                  </Typography>
                )}
              </CardContent>
              <CardActions sx={{ display: 'flex', flexDirection: 'row', justifyContent: 'space-between', px: 2, pb: 1 }}>
                <Button
                  size="small"
                  color="primary"
                  onClick={() => handlePlayChannel(channel)}
                  startIcon={<PlayArrow />}
                >
                  Oynat
                </Button>
                <Button
                  size="small"
                  onClick={() => handleToggleFavorite(channel)}
                  startIcon={favorites.some(f => f.url === channel.url) ? <Star /> : <StarBorder />}
                >
                  {favorites.some(f => f.url === channel.url) ? 'Çıkar' : 'Ekle'}
                </Button>
              </CardActions>
            </Card>
          ))}
        </Box>
      );
    }

    if (!channels || channels.length === 0) {
      return (
        <Alert severity="info">
          Bu kategoride kanal bulunamadı.
        </Alert>
      );
    }

    const channelsToShow = searchQuery.trim() ? filteredChannels : channels;
    const sortedChannels = sortChannels(channelsToShow);
    const isLiveCategory = selectedCategory?.startsWith('live') || 
                          (selectedCategory === 'favorites' && channelsToShow.some(c => c.stream_type === 'live'));
    const showSortButton = !isLiveCategory && (selectedCategory?.startsWith('movie') || selectedCategory?.startsWith('series'));

    return (
      <>
        {showSortButton && (
          <Box sx={{ display: 'flex', justifyContent: 'flex-end', mb: 2 }}>
            <Button
              startIcon={<SortIcon />}
              onClick={handleSortClick}
              variant="outlined"
            >
              Sırala
            </Button>
            <Menu
              anchorEl={sortAnchorEl}
              open={Boolean(sortAnchorEl)}
              onClose={handleSortClose}
            >
              <MenuItem 
                onClick={() => { setSortOrder('newest'); handleSortClose(); }}
                selected={sortOrder === 'newest'}
              >
                Yeniden Eskiye
              </MenuItem>
              <MenuItem 
                onClick={() => { setSortOrder('oldest'); handleSortClose(); }}
                selected={sortOrder === 'oldest'}
              >
                Eskiden Yeniye
              </MenuItem>
              <MenuItem 
                onClick={() => { setSortOrder('rating'); handleSortClose(); }}
                selected={sortOrder === 'rating'}
              >
                IMDB Puanına Göre
              </MenuItem>
            </Menu>
          </Box>
        )}
        <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(200px, 1fr))', gap: 2, p: 2 }}>
          {sortedChannels.map((channel) => (
            <Card key={channel.id} sx={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
              <CardContent sx={{ flexGrow: 1, display: 'flex', flexDirection: 'column' }}>
                {channel.stream_icon && (
                  <Box
                    component="img"
                    src={channel.stream_icon}
                    alt={channel.name}
                    sx={{
                      width: '100%',
                      height: 120,
                      objectFit: 'contain',
                      mb: 1,
                      loading: 'lazy'
                    }}
                    loading="lazy"
                  />
                )}
                <Typography
                  gutterBottom
                  variant="subtitle1"
                  component="div"
                  sx={{
                    overflow: 'hidden',
                    textOverflow: 'ellipsis',
                    display: '-webkit-box',
                    WebkitLineClamp: 2,
                    WebkitBoxOrient: 'vertical',
                    minHeight: '3em'
                  }}
                >
                  {channel.name}
                </Typography>
                {channel.rating && (
                  <Typography variant="body2" color="text.secondary">
                    IMDB: {channel.rating}
                  </Typography>
                )}
              </CardContent>
              <CardActions sx={{ display: 'flex', flexDirection: 'row', justifyContent: 'space-between', px: 2, pb: 1 }}>
                <Button
                  size="small"
                  color="primary"
                  onClick={() => handlePlayChannel(channel)}
                  startIcon={<PlayArrow />}
                >
                  Oynat
                </Button>
                <Button
                  size="small"
                  onClick={() => handleToggleFavorite(channel)}
                  startIcon={favorites.some(f => f.url === channel.url) ? <Star /> : <StarBorder />}
                >
                  {favorites.some(f => f.url === channel.url) ? 'Çıkar' : 'Ekle'}
                </Button>
              </CardActions>
            </Card>
          ))}
        </Box>
      </>
    );
  };

  const handleThemeToggle = () => {
    const newDarkMode = !darkMode;
    setDarkMode(newDarkMode);
    localStorage.setItem('darkMode', newDarkMode.toString());
    
    // Dispatch a custom event to notify index.tsx about the theme change
    window.dispatchEvent(new CustomEvent('darkModeChange'));
    
    // Force theme change in the UI
    document.documentElement.style.setProperty('color-scheme', newDarkMode ? 'dark' : 'light');
  };

  const handleAboutOpen = () => {
    setAboutDialogOpen(true);
  };

  const handleAboutClose = () => {
    setAboutDialogOpen(false);
  };

  return (
    <Box sx={{ display: 'flex', minHeight: '100vh' }}>
      <CssBaseline />
      {/* Hata mesajlarını göstermek için snackbar */}
      {error && (
        <Alert 
          severity="error" 
          onClose={() => setError(null)}
          sx={{ 
            position: 'fixed', 
            bottom: 16, 
            right: 16, 
            zIndex: (theme) => theme.zIndex.drawer + 1 
          }}
        >
          {error}
        </Alert>
      )}
      <AppBar
        position="fixed"
        sx={{
          width: { sm: `calc(100% - ${drawerWidth}px)` },
          ml: { sm: `${drawerWidth}px` },
          zIndex: (theme) => theme.zIndex.drawer + 1,
        }}
      >
        <Toolbar>
          <IconButton
            color="inherit"
            aria-label="open drawer"
            edge="start"
            onClick={handleDrawerToggle}
            sx={{ mr: 2, display: { sm: 'none' } }}
          >
            <MenuIcon />
          </IconButton>
          <Typography variant="h6" noWrap component="div" sx={{ flexGrow: 1 }}>
            Remote IPTV Player
          </Typography>
          <Box sx={{ flexGrow: 1, mx: 2 }}>
             <TextField
               size="small"
               placeholder="Kanal Ara..."
               value={searchQuery}
               onChange={(e) => setSearchQuery(e.target.value)}
               sx={{
                 width: { xs: '100%', sm: '300px', md: '400px' },
                 backgroundColor: 'rgba(255, 255, 255, 0.15)',
                 borderRadius: 1,
                 '& .MuiOutlinedInput-root': {
                   color: 'white',
                   '& fieldset': {
                     borderColor: 'transparent',
                   },
                   '&:hover fieldset': {
                     borderColor: 'rgba(255, 255, 255, 0.3)',
                   },
                   '&.Mui-focused fieldset': {
                     borderColor: 'rgba(255, 255, 255, 0.5)',
                   },
                 },
                 '& .MuiInputBase-input::placeholder': {
                   color: 'rgba(255, 255, 255, 0.7)',
                 },
               }}
               InputProps={{
                 startAdornment: (
                   <InputAdornment position="start">
                     <SearchIcon sx={{ color: 'rgba(255, 255, 255, 0.7)' }} />
                   </InputAdornment>
                 ),
               }}
             />
           </Box>
          <IconButton color="inherit" onClick={handleThemeToggle}>
            {darkMode ? <LightModeIcon /> : <DarkModeIcon />}
          </IconButton>
          <IconButton color="inherit" onClick={handleAboutOpen}>
            <InfoIcon />
          </IconButton>
          <IconButton color="inherit" onClick={() => setSettingsOpen(true)}>
            <SettingsIcon />
          </IconButton>
        </Toolbar>
      </AppBar>
      <Box
        component="nav"
        sx={{ width: { sm: drawerWidth }, flexShrink: { sm: 0 } }}
      >
        <Drawer
          variant="temporary"
          open={mobileOpen}
          onClose={handleDrawerToggle}
          ModalProps={{
            keepMounted: true,
          }}
          sx={{
            display: { xs: 'block', sm: 'none' },
            '& .MuiDrawer-paper': { boxSizing: 'border-box', width: drawerWidth },
          }}
        >
          {drawer}
        </Drawer>
        <Drawer
          variant="permanent"
          sx={{
            display: { xs: 'none', sm: 'block' },
            '& .MuiDrawer-paper': { boxSizing: 'border-box', width: drawerWidth },
          }}
          open
        >
          {drawer}
        </Drawer>
      </Box>
      <Box
        component="main"
        sx={{
          flexGrow: 1,
          p: 3,
          width: { sm: `calc(100% - ${drawerWidth}px)` },
          mt: { xs: 7, sm: 8 },
        }}
      >
        <Container>
          {error && (
            <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError(null)}>
              {error}
            </Alert>
          )}

          {loading ? (
            <Box sx={{ display: 'flex', justifyContent: 'center', my: 4 }}>
              <CircularProgress />
            </Box>
          ) : (
            <>
              {currentChannel && (
                <Box sx={{ mb: 4 }}>
                  <Typography variant="h5" gutterBottom>
                    Şu an oynatılıyor: {currentChannel.name}
                  </Typography>
                  <Button
                    variant="contained"
                    color="secondary"
                    startIcon={<Stop />}
                    onClick={handleStopChannel}
                  >
                    Durdur
                  </Button>
                </Box>
              )}

              {!selectedCategory && !searchQuery.trim() && !loading && (
                <Box sx={{ textAlign: 'center', my: 4 }}>
                  <Typography variant="h6" color="textSecondary">
                    Lütfen yandan bir kategori seçin veya arama yapın
                  </Typography>
                </Box>
              )}

              {renderChannelList()}
            </>
          )}
        </Container>
      </Box>
      <XtreamSettingsDialog
        open={settingsOpen}
        onClose={() => setSettingsOpen(false)}
        onSave={handleSaveSettings}
        initialSettings={xtreamSettings || undefined}
      />
      <Dialog open={aboutDialogOpen} onClose={handleAboutClose}>
        <DialogTitle>Hakkında</DialogTitle>
        <DialogContent>
          <DialogContentText>
            <Typography paragraph>
              by Serkan KOCAMAN & AI
            </Typography>
            <Link 
              href="https://github.com/KiPSOFT" 
              target="_blank" 
              rel="noopener noreferrer"
              color="primary"
            >
              github.com/KiPSOFT
            </Link>
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={handleAboutClose}>Kapat</Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}

export default App;

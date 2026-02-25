import { useState, useEffect } from 'react';
import { Monitor, ChevronRight } from 'lucide-react';
import { EventsOn } from '../../wailsjs/runtime';
import { GetMonitorWallpapers } from '../../wailsjs/go/app/App';
import { cn } from '@/lib/utils';
import { store } from '../../wailsjs/go/models';

interface MonitorWallpaperInfo {
  monitor_id: number;
  monitor_name: string;
  history_item: store.HistoryItem;
  thumbnail_url: string;
}

interface MonitorSwitcherProps {
  onSelect: (item: store.HistoryItem) => void;
  currentImage: store.HistoryItem | null;
}

export function MonitorSwitcher({ onSelect, currentImage }: MonitorSwitcherProps) {
  const [monitorWallpapers, setMonitorWallpapers] = useState<MonitorWallpaperInfo[]>([]);
  const [hoveredId, setHoveredId] = useState<number | null>(null);

  const refreshMonitors = async () => {
    try {
      const data = await GetMonitorWallpapers();
      setMonitorWallpapers(data || []);
    } catch (err) {
      console.error('Failed to get monitor wallpapers:', err);
    }
  };

  useEffect(() => {
    refreshMonitors();

    const unregister = EventsOn('monitor-wallpapers-changed', () => {
      refreshMonitors();
    });

    const unregisterSync = EventsOn('current-image-changed', () => {
      refreshMonitors();
    });

    return () => {
      unregister();
      unregisterSync();
    };
  }, []);

  if (monitorWallpapers.length <= 1) return null;

  return (
    <div className="absolute right-6 top-1/2 -translate-y-1/2 z-50 flex flex-col gap-4 p-3 rounded-2xl bg-black/20 backdrop-blur-xl border border-white/10 shadow-2xl items-center transition-all duration-500 hover:bg-black/30">
      {monitorWallpapers.map((mw) => {
        const isActive = currentImage?.key === mw.history_item?.key;
        
        return (
          <div 
            key={mw.monitor_id}
            className="relative group flex flex-col items-center"
            onMouseEnter={() => setHoveredId(mw.monitor_id)}
            onMouseLeave={() => setHoveredId(null)}
          >
            {/* Monitor Label */}
            <div className={cn(
                "absolute right-full mr-3 px-3 py-1 rounded-full bg-white/10 backdrop-blur-md border border-white/10 text-[10px] text-white font-medium transition-all duration-300 opacity-0 whitespace-nowrap top-1/2 -translate-y-1/2 pointer-events-none",
                hoveredId === mw.monitor_id && "opacity-100 mr-4"
            )}>
               显示器 {mw.monitor_id + 1}: {mw.monitor_name}
            </div>

            {/* Thumbnail */}
            <button
              onClick={() => mw.history_item && onSelect(mw.history_item)}
              className={cn(
                "relative w-32 aspect-video rounded-lg overflow-hidden border-2 transition-all duration-300 shadow-lg",
                isActive 
                  ? "border-indigo-500 scale-105 z-10 shadow-indigo-500/20" 
                  : "border-transparent opacity-70 hover:opacity-100 hover:scale-105"
              )}
            >
              {mw.thumbnail_url ? (
                <img 
                  src={mw.thumbnail_url} 
                  alt={mw.history_item?.title} 
                  className="w-full h-full object-cover"
                />
              ) : (
                <div className="w-full h-full bg-slate-800 flex items-center justify-center">
                  <Monitor className="h-6 w-6 text-slate-500" />
                </div>
              )}
              
              {/* Active Indicator Dot */}
              {isActive && (
                <div className="absolute top-1.5 right-1.5 w-2 h-2 rounded-full bg-indigo-500 shadow-sm" />
              )}

              {/* Title Overlay on Hover */}
              <div className="absolute inset-0 bg-black/40 opacity-0 group-hover:opacity-100 transition-opacity flex items-center justify-center">
                 <span className="text-[10px] text-white font-medium px-2 text-center line-clamp-2">
                    {mw.history_item?.title || '未设置'}
                 </span>
              </div>
            </button>
            
            {/* Monitor Icon/ID */}
            <div className={cn(
                "mt-2 text-[10px] font-bold transition-colors duration-300 flex items-center gap-1",
                isActive ? "text-indigo-400" : "text-white/40 group-hover:text-white/70"
            )}>
              <Monitor className="h-3 w-3" />
              <span>{mw.monitor_id + 1}</span>
            </div>
          </div>
        );
      })}
    </div>
  );
}

import { useState, useEffect } from 'react';
import { Image as ImageIcon, CheckCircle2, Trash2, ChevronUp } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { 
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  DropdownMenuSeparator,
} from '@/components/ui/dropdown-menu';
import { cn } from '@/lib/utils';
import { store } from '../../wailsjs/go/models';
import { GetThumbnailURL } from '../../wailsjs/go/app/App';

interface HistoryItemCardProps {
  item: store.HistoryItem;
  monitors: any[];
  onApply: (monitorId?: number) => void;
  onDelete: () => void;
}

export function HistoryItemCard({ item, monitors, onApply, onDelete }: HistoryItemCardProps) {
  const [thumb, setThumb] = useState<string>('');
  const [isLoaded, setIsLoaded] = useState(false);
  const [retryCount, setRetryCount] = useState(0);
  const hasMultipleMonitors = monitors && monitors.length > 1;

  useEffect(() => {
    let cancelled = false;
    setIsLoaded(false);
    setRetryCount(0);
    setThumb('');

    GetThumbnailURL(item.image_path)
      .then((url) => {
        if (!cancelled) {
          setThumb(url);
        }
      })
      .catch(console.error);

    return () => {
      cancelled = true;
    };
  }, [item.image_path]);

  const thumbSrc = thumb
    ? `${thumb}${thumb.includes('?') ? '&' : '?'}retry=${retryCount}`
    : '';

  return (
    <div className="group relative overflow-hidden rounded-xl border bg-card text-card-foreground shadow-md hover:shadow-2xl transition-all duration-500 ease-in-out">
      <div className="aspect-video w-full overflow-hidden bg-black relative">
        {thumb ? (
          <>
            <img 
              src={thumbSrc}
              alt={item.title} 
              onLoad={() => setIsLoaded(true)}
              onError={() => {
                setIsLoaded(false);
                if (retryCount >= 2) {
                  setThumb('');
                  return;
                }
                setRetryCount((prev) => prev + 1);
              }}
              className={cn(
                "h-full w-full object-contain transition-all duration-700 group-hover:scale-105",
                isLoaded ? "opacity-100 scale-100" : "opacity-0 scale-105"
              )} 
            />
            {!isLoaded && (
              <div className="absolute inset-0 flex items-center justify-center animate-pulse bg-muted">
                <ImageIcon className="h-8 w-8 text-muted-foreground/20" />
              </div>
            )}
          </>
        ) : (
          <div className="h-full w-full flex items-center justify-center">
            <ImageIcon className="h-8 w-8 text-muted-foreground/50" />
          </div>
        )}
      </div>
      
      {/* Information Overlay on Hover */}
      <div className="absolute inset-0 bg-gradient-to-t from-black/90 via-black/40 to-transparent opacity-0 group-hover:opacity-100 transition-all duration-300 flex flex-col justify-end p-4">
        <div className="translate-y-8 group-hover:translate-y-0 transition-transform duration-500 ease-out">
          <h4 className="text-sm font-bold text-white mb-1.5 line-clamp-1 drop-shadow-sm">{item.title}</h4>
          <p className="text-[10px] text-white/80 line-clamp-2 mb-3 leading-snug h-8 italic">
            {item.copyright}
          </p>
          <div className="flex items-center gap-2 mb-4">
             <span className="text-[9px] bg-white/20 px-2 py-0.5 rounded-full text-white backdrop-blur-md border border-white/10">
               {item.date}
             </span>
             <span className="text-[9px] bg-indigo-500/40 px-2 py-0.5 rounded-full text-white backdrop-blur-md border border-white/10">
               {item.chosen_variant}
             </span>
          </div>
          <div className="flex gap-2">
            {hasMultipleMonitors ? (
              <div className="flex flex-1 gap-0">
                <Button 
                  size="sm" 
                  className="h-9 flex-1 bg-white text-black hover:bg-white/90 text-xs font-medium rounded-l-lg rounded-r-none shadow-lg border-r border-black/10" 
                  onClick={() => onApply()}
                >
                  <CheckCircle2 className="h-3.5 w-3.5 mr-1.5" />
                  全部
                </Button>
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button 
                      size="sm" 
                      className="h-9 px-2 bg-white text-black hover:bg-white/90 rounded-r-lg rounded-l-none shadow-lg"
                    >
                      <ChevronUp className="h-3.5 w-3.5" />
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end" className="w-40">
                    <DropdownMenuItem onClick={() => onApply()}>
                      所有显示器
                    </DropdownMenuItem>
                    <DropdownMenuSeparator />
                    {monitors.map((m) => (
                      <DropdownMenuItem key={m.ID} onClick={() => onApply(m.ID)}>
                        显示器 {m.ID + 1}
                      </DropdownMenuItem>
                    ))}
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>
            ) : (
              <Button 
                size="sm" 
                className="h-9 flex-1 bg-white text-black hover:bg-white/90 text-xs font-medium rounded-lg shadow-lg" 
                onClick={() => onApply()}
              >
                <CheckCircle2 className="h-3.5 w-3.5 mr-1.5" />
                应用壁纸
              </Button>
            )}
            <Button 
              size="sm" 
              variant="destructive" 
              className="h-9 px-3 rounded-lg shadow-lg bg-red-500/80 hover:bg-red-600" 
              onClick={onDelete}
            >
              <Trash2 className="h-3.5 w-3.5" />
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}

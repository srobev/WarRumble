import React, { useState, useEffect } from 'react';
import { WebSocketClient } from '../systems/WebSocketClient';

interface MapBrowserProps {
    isOpen: boolean;
    onClose: () => void;
    wsClient?: WebSocketClient;
    onMapSelected: (mapId: string) => void;
}

const availableMaps = [
    'colosseum',
    'east_gate',
    'mid_bridge',
    'north_tower',
    'south_gate',
    'west_keep',
    'tester',
    'new-map'
];

export const MapBrowser: React.FC<MapBrowserProps> = ({
    isOpen,
    onClose,
    wsClient,
    onMapSelected
}) => {
    const [selectedIndex, setSelectedIndex] = useState(0);

    useEffect(() => {
        if (isOpen) {
            setSelectedIndex(0);
        }
    }, [isOpen]);

    const handleKeyDown = (e: React.KeyboardEvent) => {
        if (e.key === 'ArrowUp') {
            setSelectedIndex(prev => Math.max(0, prev - 1));
        } else if (e.key === 'ArrowDown') {
            setSelectedIndex(prev => Math.min(availableMaps.length - 1, prev + 1));
        } else if (e.key === 'Enter') {
            handleLoadMap();
        } else if (e.key === 'Escape') {
            onClose();
        }
    };

    const handleLoadMap = () => {
        const mapId = availableMaps[selectedIndex];
        if (wsClient) {
            wsClient.requestMap(mapId);
        }
        onMapSelected(mapId);
        onClose();
    };

    if (!isOpen) return null;

    return (
        <div className="fixed inset-0 bg-black bg-opacity-70 flex items-center justify-center z-50">
            <div className="bg-gray-800 border border-gray-600 rounded-lg p-6 min-w-96 max-w-lg">
                {/* Header */}
                <div className="flex justify-between items-center mb-4">
                    <h2 className="text-xl font-bold text-white">Load Map</h2>
                    <button
                        onClick={onClose}
                        className="text-gray-400 hover:text-white w-6 h-6 flex items-center justify-center text-sm border border-gray-600 rounded hover:bg-gray-700"
                    >
                        ✕
                    </button>
                </div>

                {/* Map List */}
                <div className="mb-4 max-h-64 overflow-y-auto">
                    {availableMaps.map((mapId, index) => (
                        <div
                            key={mapId}
                            className={`p-2 mb-1 cursor-pointer rounded border ${
                                index === selectedIndex
                                    ? 'bg-blue-600 border-blue-500 text-white'
                                    : 'bg-gray-700 border-gray-600 text-gray-300 hover:bg-gray-600'
                            }`}
                            onClick={() => setSelectedIndex(index)}
                            onDoubleClick={handleLoadMap}
                            tabIndex={0}
                            onKeyDown={handleKeyDown}
                        >
                            {mapId.replace('_', ' ')}
                        </div>
                    ))}
                </div>

                {/* Actions */}
                <div className="flex gap-3">
                    <button
                        onClick={handleLoadMap}
                        className="flex-1 bg-blue-600 hover:bg-blue-700 text-white py-2 px-4 rounded border border-blue-500"
                    >
                        Load Map
                    </button>
                    <button
                        onClick={onClose}
                        className="flex-1 bg-gray-600 hover:bg-gray-700 text-white py-2 px-4 rounded border border-gray-500"
                    >
                        Cancel
                    </button>
                </div>

                {/* Help Text */}
                <div className="mt-4 text-xs text-gray-400">
                    <p>Use ↑/↓ arrows to navigate, Enter to load, Escape to cancel</p>
                    <p>Double-click or press Enter to load selected map</p>
                </div>
            </div>
        </div>
    );
};

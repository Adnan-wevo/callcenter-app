import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';

import { ApiService } from '../../core/api/api.service';

/** Mirrors handlers.settingRow. */
export interface SettingRow {
  key: string;
  value: string;
  description: string;
}

@Injectable({ providedIn: 'root' })
export class SettingsService {
  private readonly api = inject(ApiService);

  categories(): Observable<string[]> {
    return this.api.get<string[]>('secure/call-center/settings/categories');
  }

  list(category: string): Observable<SettingRow[]> {
    return this.api.get<SettingRow[]>(`secure/call-center/settings/${category}`);
  }

  update(category: string, values: Record<string, string>): Observable<{ status: string }> {
    return this.api.put<{ status: string }>(`secure/call-center/settings/${category}`, { values });
  }
}
